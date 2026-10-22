package seedance

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func taskView(task *model.Task, openai bool) any {
	if openai {
		return task.ToOpenAIVideo()
	}
	var data map[string]any
	_ = common.Unmarshal(task.Data, &data)
	if data == nil {
		data = map[string]any{}
	}
	data["id"] = task.TaskID
	// Procurement fields belong to administrators, not the customer task API.
	for _, key := range []string{"charge_cents", "is_charged", "asset_auto_ingest"} {
		delete(data, key)
	}
	data["model"] = task.Properties.OriginModelName
	if _, ok := data["status"]; !ok {
		data["status"] = map[model.TaskStatus]string{model.TaskStatusNotStart: "queued", model.TaskStatusSubmitted: "queued", model.TaskStatusQueued: "queued", model.TaskStatusInProgress: "running", model.TaskStatusSuccess: "succeeded", model.TaskStatusFailure: "failed"}[task.Status]
	}
	return data
}
func ListTasks(c *gin.Context) {
	if _, ok := configForRequest(c); !ok {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 || page > 100000 || size < 1 || size > 200 {
		writeError(c, apiError(400, "invalid_pagination", "Page must be 1..100000 and page_size 1..200"))
		return
	}
	q := model.DB.Model(&model.Task{}).Where("user_id = ? AND platform = ?", userID(c), "doubao").Where("id NOT IN (?)", model.DB.Model(&model.JDSeedanceTaskDeletion{}).Select("task_id"))
	ids := c.QueryArray("filter.task_ids")
	if len(ids) == 1 {
		ids = strings.Split(ids[0], ",")
	}
	if len(ids) > 100 {
		writeError(c, apiError(400, "invalid_filter", "At most 100 task IDs can be queried"))
		return
	}
	if len(ids) > 0 {
		q = q.Where("task_id IN ?", ids)
	}
	// Native statuses map to the host's portable indexed status column.
	states := map[string]model.TaskStatus{"queued": model.TaskStatusQueued, "running": model.TaskStatusInProgress, "succeeded": model.TaskStatusSuccess, "failed": model.TaskStatusFailure, "cancelled": model.TaskStatusFailure, "expired": model.TaskStatusFailure}
	status := c.Query("filter.status")
	if status != "" {
		s, ok := states[status]
		if !ok {
			writeError(c, apiError(400, "invalid_filter", "Unsupported status filter"))
			return
		}
		if status == "queued" {
			q = q.Where("status IN ?", []model.TaskStatus{model.TaskStatusQueued, model.TaskStatusSubmitted, model.TaskStatusNotStart})
		} else {
			q = q.Where("status = ?", s)
		}
	}
	// Scan bounded pages locally so JSON fields remain portable on all databases.
	// Use an exact model/status match before pagination, including cancelled vs failed.
	var rows []*model.Task
	items := []any{}
	total := 0
	modelName := c.Query("filter.model")
	openai := c.Request.URL.Path == "/v1/videos"
	err := q.Order("id ASC").FindInBatches(&rows, 200, func(_tx *gorm.DB, _batch int) error {
		for _, task := range rows {
			if !task.ResultRetrievable() {
				continue
			}
			if modelName != "" && task.Properties.OriginModelName != modelName {
				continue
			}
			var data map[string]any
			_ = common.Unmarshal(task.Data, &data)
			if tier := c.Query("filter.service_tier"); tier != "" && data["service_tier"] != tier {
				continue
			}
			if status == "cancelled" || status == "failed" || status == "expired" {
				s, _ := data["status"].(string)
				if s == "canceled" {
					s = "cancelled"
				}
				if s == "" {
					s = "failed"
				}
				if s != status {
					continue
				}
			}
			if total >= (page-1)*size && len(items) < size {
				items = append(items, taskView(task, openai))
			}
			total++
		}
		return nil
	}).Error
	if err != nil {
		writeError(c, err)
		return
	}
	if openai {
		c.JSON(200, gin.H{"object": "list", "data": items, "has_more": page*size < total, "total": total})
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": page, "page_size": size})
}
func DeleteTask(c *gin.Context) {
	cfg, ok := configForRequest(c)
	if !ok {
		return
	}
	id := c.Param("task_id")
	task, found, err := model.GetByTaskId(userID(c), id)
	if err != nil {
		writeError(c, err)
		return
	}
	if !found || task == nil || task.Platform != "doubao" || !task.ResultRetrievable() {
		writeError(c, apiError(404, "not_found", "Task not found"))
		return
	}
	if task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
		if err = model.HideJDSeedanceTask(userID(c), id); err != nil {
			writeError(c, err)
			return
		}
		c.JSON(200, gin.H{"id": id, "deleted": true, "upstream_deleted": false, "billing_records_retained": true})
		return
	}
	var binding Binding
	var ch *model.Channel
	for _, b := range cfg.Bindings {
		if b.ChannelID == task.ChannelId {
			candidate, channel, e := ResolveBinding(c, cfg, b.Name, task.Properties.OriginModelName)
			if e == nil {
				binding = candidate
				ch = channel
				break
			}
		}
	}
	if ch == nil || !binding.Models[task.Properties.OriginModelName].Cancel {
		writeError(c, apiError(422, "cancel_unsupported", "Cancellation is not enabled for this task"))
		return
	}
	// Generation already saves the exact key for polling. Do not send a DELETE
	// with a different account after an administrator rotates channel credentials.
	if task.PrivateData.SeedanceAccountHash == "" || task.PrivateData.SeedanceAccountHash != AccountHash(ch) || strings.TrimSpace(task.PrivateData.Key) != strings.TrimSpace(ch.Key) {
		writeError(c, apiError(409, "account_changed", "Task account provenance is missing or has changed"))
		return
	}
	upstreamID := task.GetUpstreamTaskID()
	if upstreamID == "" || len(upstreamID) > 256 || upstreamID == "." || upstreamID == ".." || strings.ContainsAny(upstreamID, "/?#") {
		writeError(c, apiError(409, "invalid_task_identity", "Task upstream identity is invalid"))
		return
	}
	_, err = Call(c.Request.Context(), ch, http.MethodDelete, "/api/v3/contents/generations/tasks/"+url.PathEscape(task.GetUpstreamTaskID()), nil)
	if err != nil {
		writeError(c, err)
		return
	}
	// The existing poller owns terminal CAS, usage settlement and refunds. DELETE
	// acknowledgement alone must never credit a wallet or fabricate cancellation.
	upstream := task.GetUpstreamTaskID()
	err = service.UpdateVideoTasks(c.Request.Context(), task.Platform, map[int][]string{task.ChannelId: {upstream}}, map[string]*model.Task{upstream: task})
	if err != nil {
		writeError(c, err)
		return
	}
	latest, found, err := model.GetByTaskId(userID(c), id)
	if err != nil || !found {
		writeError(c, apiError(502, "cancel_unconfirmed", "Cancellation result could not be read"))
		return
	}
	var data map[string]any
	_ = common.Unmarshal(latest.Data, &data)
	status, _ := data["status"].(string)
	if status == "cancelled" || status == "canceled" {
		c.JSON(200, gin.H{"id": id, "status": "cancelled", "deleted": false})
		return
	}
	if latest.Status == model.TaskStatusSuccess || latest.Status == model.TaskStatusFailure {
		c.JSON(409, gin.H{"error": gin.H{"code": "task_already_terminal", "message": TranslateErrorMessage(c, "Task completed before cancellation was confirmed")}, "task": taskView(latest, false)})
		return
	}
	c.JSON(202, gin.H{"id": id, "status": "cancel_requested", "deleted": false})
}
