"""JDOTOKEN customer documentation. Static content; never load production secrets."""
from html import escape
import json

BASE = 'https://www.jdotoken.com'
VIDEO = '/doubao/api/v3/contents/generations/tasks'
MINI = 'doubao-seedance-2-0-mini-260615'
FULL = 'doubao-seedance-2-0-260128'
SECTIONS = []
EXAMPLES = []


def code(language, source):
    source = source.strip()
    EXAMPLES.append({'language': language, 'source': source})
    return f'<div class="code-block"><div class="code-toolbar"><span>{escape(language)}</span><button class="copy" type="button" aria-label="复制 {escape(language)} 代码">复制</button></div><pre><code>{escape(source)}</code></pre></div>'


def table(headers, rows):
    return '<div class="table-scroll"><table><thead><tr>' + ''.join(f'<th scope="col">{x}</th>' for x in headers) + '</tr></thead><tbody>' + ''.join('<tr>' + ''.join(f'<td>{x}</td>' for x in row) + '</tr>' for row in rows) + '</tbody></table></div>'


def note(text):
    return f'<aside class="note">{text}</aside>'


def section(group, key, title, description, body):
    SECTIONS.append(dict(group=group, key=key, title=title, description=description, body=body))


def curl(path, payload=None, extra=''):
    lines = [f'curl --fail-with-body --show-error{extra} "{BASE}{path}" \\', '  -H "Authorization: Bearer $JDOTOKEN_API_KEY"']
    if payload is not None:
        lines[-1] += ' \\'
        lines += ['  -H "Content-Type: application/json" \\', "  -d '" + json.dumps(payload, ensure_ascii=False, indent=2) + "'"]
    return '\n'.join(lines)


section('开始使用', 'overview', '接入概览', '一份 API 密钥，连接对话与视频生成。', '''
<p>使用 JDOTOKEN 将模型能力接入你的应用。对话请求直接返回回答，也可逐段流式返回；视频请求会先创建任务，再通过任务 ID 查询生成结果。</p>
<div class="base-address"><span>网关地址</span><code>https://www.jdotoken.com</code><span class="badge">HTTPS</span></div>
<div class="paths"><a href="#chat"><span class="method">POST</span><code>/v1/chat/completions</code><span>对话 →</span></a><a href="#create-video"><span class="method">POST</span><code>/doubao/api/v3/contents/generations/tasks</code><span>视频 →</span></a></div>
<p>第一次接入，先创建密钥并查询可用模型，再选择下方的对话或视频示例。所有示例使用本站地址与本站模型名称；https://jdotoken.com 也支持相同接口，建议统一使用主域名。</p>
<div class="text-links"><a href="#quickstart">开始第一次调用 ↗</a><a href="#video-workflow">了解视频任务流程 ↗</a></div>
''')

section('开始使用', 'quickstart', '快速开始', '从创建密钥到发出第一条请求。', '''
<ol class="steps"><li><strong>创建 API 密钥</strong><p>登录<a href="/keys">控制台 → API 密钥</a>，创建令牌并确认余额、额度、有效期及模型权限。复制密钥并保存在服务端。</p></li><li><strong>设置环境变量</strong><p>以下命令用于本地开发。生产环境请使用部署平台的密钥管理功能。</p></li></ol>
''' + code('bash', 'export JDOTOKEN_API_KEY="替换为你的本站 API 密钥"') + '''
<h3 id="first-request">查询可用模型</h3><p>这是一个只读请求，可用于检查接入地址与鉴权。选择响应 <code>data</code> 中的模型 <code>id</code>，不要填写模型展示名称或供应商内部别名。</p>
''' + code('bash', curl('/v1/models')) + '''
<p>随后使用<a href="#chat">对话接口</a>或<a href="#create-video">视频接口</a>。生成调用会按账户当前价格计费。</p>
''' + note('接入地址不要混用：OpenAI SDK 的 <code>base_url</code> 为 <code>https://www.jdotoken.com/v1</code>；Seedance 原生接口的完整前缀为 <code>https://www.jdotoken.com/doubao/api/v3</code>。'))

section('开始使用', 'authentication', '鉴权与 API 密钥', '业务 API 使用你的 JDOTOKEN 密钥；签名文件传输使用临时签名。', '''
<p>通用接口使用 <code>Authorization: Bearer &lt;API_KEY&gt;</code>。Anthropic 消息接口也接受 <code>x-api-key</code>。密钥权限取决于账户、令牌分组、模型限制、额度、有效期等设置。</p>
''' + code('http', 'Authorization: Bearer YOUR_JDOTOKEN_API_KEY\nContent-Type: application/json') + '''
<ul><li>不要将密钥写进公开仓库、浏览器前端代码或 URL 查询参数。</li><li>开发、测试与生产分别使用令牌，并设置适当的额度与有效期。</li><li>密钥泄露后，在控制台撤销并重新创建。</li><li>任务查询和下载也需要鉴权；任务必须属于当前用户。文件上传与签名下载的 PUT/GET/HEAD 使用临时签名，不额外携带 API Key。</li></ul>
''')

section('开始使用', 'models', '模型目录', '可用性以当前账户的模型列表为准。', '''
<p>使用 <code>GET /v1/models</code> 获取你可以访问的模型。本站配置覆盖以下模型家族，但令牌或分组限制可能让你看到的列表更少。出现在列表中不表示每一种协议、工具或多模态参数都受支持。</p>
''' + table(['模型家族', '请求名称示例', '主要接入方式'], [
    ['Claude', '<code>claude-sonnet-4-6</code>', 'Anthropic Messages；其他协议按模型支持情况'],
    ['DeepSeek', '<code>deepseek-v4-flash</code>', 'OpenAI Chat Completions'],
    ['Kimi', '<code>kimi-k3</code>', 'OpenAI Chat Completions'],
    ['GLM', '<code>glm-5.3</code>', 'OpenAI Chat Completions'],
    ['Seedance', f'<code>{MINI}</code>', '异步视频任务'],
]) + '''<p>价格与可用分组请查看<a href="/pricing">模型广场</a>；模型名称及价格可能调整。下列对话示例是协议接入示例，不代表每个已配置模型均已逐项实测。</p>''')

chat = {'model': 'deepseek-v4-flash', 'messages': [{'role': 'user', 'content': '用一句话介绍你能帮助我做什么。'}], 'stream': False}
chat_curl = code('bash', curl('/v1/chat/completions', chat))
chat_python = code('python', '''import json
import os
from urllib.request import Request, urlopen

payload = {
    "model": "deepseek-v4-flash",
    "messages": [{"role": "user", "content": "用一句话介绍你能帮助我做什么。"}],
    "stream": False,
}
request = Request(
    "https://www.jdotoken.com/v1/chat/completions",
    data=json.dumps(payload).encode(),
    headers={"Authorization": "Bearer " + os.environ["JDOTOKEN_API_KEY"],
             "Content-Type": "application/json"},
)
with urlopen(request, timeout=120) as response:
    result = json.load(response)
print(result["choices"][0]["message"]["content"])
''')
chat_js = code('javascript', '''// Node.js 18+，在服务端执行。
const response = await fetch("https://www.jdotoken.com/v1/chat/completions", {
  method: "POST",
  headers: {
    Authorization: `Bearer ${process.env.JDOTOKEN_API_KEY}`,
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    model: "deepseek-v4-flash",
    messages: [{ role: "user", content: "用一句话介绍你能帮助我做什么。" }],
    stream: false,
  }),
  signal: AbortSignal.timeout(120000),
});
if (!response.ok) throw new Error(`HTTP ${response.status}: ${await response.text()}`);
const result = await response.json();
console.log(result.choices[0].message.content);
''')
section('对话 API', 'chat', '创建对话', '使用 OpenAI 兼容的 Chat Completions 格式。', '''
<div class="endpoint"><span class="method">POST</span><code>/v1/chat/completions</code></div>
<p>设置 <code>model</code> 和 <code>messages</code>，从响应 <code>choices[0].message.content</code> 读取文本。请把示例模型替换成账户实际可用的模型。</p>
<div class="code-tabs"><div role="tablist" aria-label="对话代码语言"><button role="tab" id="tab-curl" aria-controls="panel-curl" aria-selected="true">cURL</button><button role="tab" id="tab-python" aria-controls="panel-python" aria-selected="false" tabindex="-1">Python</button><button role="tab" id="tab-js" aria-controls="panel-js" aria-selected="false" tabindex="-1">JavaScript</button></div>
''' + f'<div role="tabpanel" id="panel-curl" aria-labelledby="tab-curl">{chat_curl}</div><div role="tabpanel" id="panel-python" aria-labelledby="tab-python" hidden>{chat_python}</div><div role="tabpanel" id="panel-js" aria-labelledby="tab-js" hidden>{chat_js}</div></div>' + '''
<h3 id="sdk-base">使用已有 OpenAI 客户端</h3><p>将客户端的 API Key 设置为本站密钥，Base URL 设置为 <code>https://www.jdotoken.com/v1</code>，模型名称设置为本站返回的 <code>id</code>。工具调用、JSON 输出、推理参数和多模态输入需确认所选模型支持。</p>
''')

section('对话 API', 'streaming', '流式输出', '边生成，边接收文本。', '''
<p>把请求中的 <code>stream</code> 设置为 <code>true</code>。使用 cURL 时加 <code>-N</code> 关闭输出缓冲。</p>
''' + code('bash', curl('/v1/chat/completions', {**chat, 'stream': True}, ' -N')) + '''
<p>Chat Completions 的流式响应使用 SSE，读取各个 <code>data:</code> 事件中的 <code>choices[].delta</code>，直到收到 <code>[DONE]</code> 或连接结束。不同事件可能只包含角色、工具调用、结束原因或用量，不能假设每个事件都有文本。</p>
''' + note('流开始前的错误可能是 HTTP JSON 响应；开始后的异常也可能出现在事件流中。保存请求状态并设置超时，不要把网络中断直接判定为服务端未执行。'))

section('对话 API', 'anthropic', 'Claude / Anthropic', '使用 Messages 格式接入 Claude。', '''
<div class="endpoint"><span class="method">POST</span><code>/v1/messages</code></div>
<p>使用本站密钥和可用的 Claude 模型，设置 <code>anthropic-version</code>。Messages 的响应为 <code>content</code> 内容块数组，与 Chat Completions 的 <code>choices</code> 不同。</p>
''' + code('bash', r'''curl "https://www.jdotoken.com/v1/messages" \
  -H "x-api-key: $JDOTOKEN_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-6",
    "max_tokens": 256,
    "messages": [{"role": "user", "content": "请简要介绍你自己。"}]
  }' ''') + '''
<p>Anthropic 客户端的基础地址使用 <code>https://www.jdotoken.com</code>，由客户端追加 <code>/v1/messages</code>。流式事件使用 Anthropic 协议，不应套用 OpenAI 的事件解析逻辑。</p>
''')

section('视频 API', 'video-workflow', '视频任务流程', '先提交任务，再等待并获取结果。', '''
<div class="flow" aria-label="视频任务流程"><span>创建任务</span><b>→</b><span>保存 task_id</span><b>→</b><span>轮询状态</span><b>→</b><span>下载视频</span></div>
<p>Seedance 使用异步接口。创建响应中的 <code>id</code> 是本站任务 ID；后续查询和下载始终使用这个 ID。生成时间受模型和队列影响，不能将单次实测耗时当作服务时效承诺。</p>
''' + note('本站 Seedance 原生路径包含 <code>/doubao</code>：<code>/doubao/api/v3/contents/generations/tasks</code>。不要直接使用供应商文档中的根路径 <code>/api/v3/...</code>。') + '''
<p>2026-10-23 生产回归已完成：两个素材服务的十个管理 Action、素材引用生成、上传引用生成、任务列表与隐藏，以及 fast 取消。三条真实视频均完成查询、下载和解码；素材生成样本为 4 秒、480p。2026-10-24 已补充 standard mini 的图片、视频、音频入库与素材引用生成验收：图片＋音频、图片＋视频＋音频均通过本站生成，下载成品含非静音音轨。历史 mini 文生视频、Base64 首帧、公开参考视频及 2.5 草稿记录仍单独标注。</p>
''')

section('视频 API', 'video-models', 'Seedance 模型与参数', '使用本站公开的完整模型 ID。', table(['模型 ID', '分辨率档位', '时长范围', '验证范围'], [
    ['<code>doubao-seedance-2-0-260128</code>', '480p / 720p / 1080p / 4k', '4–15 秒或 -1', 'standard 15 秒、720p、9:16、4 图＋2 音频＋1 视频素材 ID 生成通过'],
    ['<code>doubao-seedance-2-0-fast-260128</code>', '480p / 720p', '4–15 秒或 -1', 'standard 图片＋视频＋音频素材 ID 生成通过，4 秒、480p；取消已实测'],
    [f'<code>{MINI}</code>', '480p / 720p', '4–15 秒或 -1', '4 秒、480p 普通视频通过；standard 图片＋音频、图片＋视频＋音频素材 ID 生成通过'],
    ['<code>doubao-seedance-2-5-260628</code>', '480p / 720p / 1080p', '4–30 秒或 -1', 'standard 图片＋视频＋音频素材 ID 正片生成通过，4 秒、480p；草稿有历史样本'],
]) + '''<p>表格中的分辨率和时长来自当前适配能力声明，并不表示所有组合都已实测。<code>-1</code> 表示由模型选择时长；首次调用建议显式设置 4 秒和 480p。具体参数组合仍受上游约束。</p><p>分辨率是生成档位，最终编码宽高可能进行对齐。例如 2.0 mini 的一次 480p 样例实际为 864×496，以下载文件为准。</p>''')

video = {'model': MINI, 'content': [{'type': 'text', 'text': '一个白色与青绿色的小机器人站在蓝色山间草地上，轻轻挥手。镜头固定，动作自然，没有字幕。'}], 'duration': 4, 'resolution': '480p', 'ratio': '16:9', 'draft': False, 'generate_audio': False, 'watermark': False, 'execution_expires_after': 3600}
section('视频 API', 'create-video', '创建视频任务', '从一条简短的文生视频请求开始。', '<div class="endpoint"><span class="method">POST</span><code>' + VIDEO + '</code></div>' + code('bash', curl(VIDEO, video)) + '''
<h3 id="video-created-response">创建响应</h3><p>下方为结构示例，ID 与时间为占位值。创建成功只表示任务已受理，请继续查询最终状态。</p>
''' + code('json', json.dumps({'id':'task_YOUR_TASK_ID','model':MINI,'status':'queued','draft':False,'created_at':1791734195}, indent=2)) + '''
<h3 id="video-fields">常用请求字段</h3>
''' + table(['字段', '类型', '说明'], [
    ['model', 'string', '必填；本站公开的模型 ID。'],
    ['content', 'array', '必填；文本、图片等内容项。文本项使用 type=text 和 text。'],
    ['duration', 'integer', '按模型限制设置时长，或用 -1 让模型选择。'],
    ['resolution', 'string', '例如 480p、720p；受模型限制。'],
    ['ratio', 'string', '例如 16:9、9:16、1:1、adaptive；受任务类型限制。'],
    ['draft', 'boolean', '普通视频为 false；仅 2.5 支持 true。mini 不支持草稿；当前服务能力校验可返回 422。'],
    ['return_last_frame', 'boolean', 'true 时请求尾帧；mini 已验证。返回的图片不保证 PNG，请检查实际 Content-Type。'],
    ['generate_audio', 'boolean', '是否请求生成音频；支持情况取决于模型及服务。'],
    ['watermark', 'boolean', '是否请求水印；上游可能有额外要求。'],
    ['execution_expires_after', 'integer', '上游任务有效时长，单位秒；不等于客户端 HTTP 超时。示例使用 3600。'],
]) + note('返回的 <code>model</code> 名称可能与提交名称格式不同。提交时仍应使用模型列表中的 ID；不要根据返回名称自行替换下一次请求的模型。'))

section('视频 API', 'references', '使用图片参考', '让视频沿用参考图片中的主体与画面。', '''
<p>在 <code>content</code> 中同时提供文本和 <code>image_url</code>，将参考图的 <code>role</code> 设为 <code>reference_image</code>。图片可使用上游能读取的 HTTPS URL；内嵌 PNG data URL 已在本站真实请求中验证。</p>
''' + code('json', json.dumps({'model': MINI, 'content': [{'type':'text','text':'参考图中的机器人缓慢挥手，镜头保持固定。'}, {'type':'image_url','role':'reference_image','image_url':{'url':'https://your-domain.example/reference.png'}}], 'duration':4,'resolution':'480p','ratio':'16:9','generate_audio':False}, ensure_ascii=False, indent=2)) + '''
<p>请替换示例图片地址。不能使用本机文件路径、localhost 地址或需要浏览器登录才能访问的图片地址。下方 Python 示例将你自己的 PNG 作为内嵌图片提交；会创建一个收费视频任务。</p>
''' + code('python', '''import base64
import json
import os
from pathlib import Path
from urllib.request import Request, urlopen

image = base64.b64encode(Path("reference.png").read_bytes()).decode()
payload = {
    "model": "doubao-seedance-2-0-mini-260615",
    "content": [
        {"type": "text", "text": "参考图中的机器人缓慢挥手，镜头保持固定。"},
        {"type": "image_url", "role": "reference_image",
         "image_url": {"url": "data:image/png;base64," + image}},
    ],
    "duration": 4, "resolution": "480p", "ratio": "16:9",
    "draft": False, "generate_audio": False, "watermark": False,
    "execution_expires_after": 3600,
}
request = Request(
    "https://www.jdotoken.com/doubao/api/v3/contents/generations/tasks",
    data=json.dumps(payload).encode(),
    headers={"Authorization": "Bearer " + os.environ["JDOTOKEN_API_KEY"],
             "Content-Type": "application/json"},
)
with urlopen(request, timeout=120) as response:
    task = json.load(response)
print(task["id"])  # 保存此 ID，再查询任务；不要再次提交来查询进度。
''') + '''<p>Base64 首帧也已验证：将 role 改为 first_frame，并使用 data:image/jpeg;base64,... 等与真实编码一致的 data URL。首帧和 reference_image 不要混用。参考视频和 asset 模式见下节；音频输入与首尾帧组合尚未验收。</p>''')

section('视频 API', 'reference-video', '使用参考视频', '参考视频提供动作和画面信息，仍需轮询生成任务。', '''
<p>以下 mini + 公网视频组合已通过本站 SDGO 服务来源实测。视频 role 使用 reference_video，输出时长由 duration 控制。该结果不代表所有模型、服务来源与素材都支持同样组合。</p>
''' + code('bash', curl(VIDEO, {**video, 'content': [{'type':'text','text':'参考视频中机器人的动作，生成一段自然挥手的画面。'}, {'type':'video_url','role':'reference_video','video_url':{'url':'https://your-domain.example/reference.mp4'}}]})) + '<p>替换成可被上游直接读取的有效 MP4 地址；本机路径和需要登录的页面地址不可用。此公网视频示例未验收视频 Base64 或音频组合；standard mini 的音频素材 ID 生成见<a href="#audio-reference">音频素材章节</a>。</p>')

ASSETS = '/doubao/api/v3/assets/?Action={}&Version=2024-01-01'
section('素材与上传', 'capabilities', '查询服务与模型能力', '先查询当前 API Key 可用的服务，再选择素材流程。', code('bash', curl('/v1/seedance/capabilities')) + """
<p>响应只列出当前令牌有权限使用的服务和模型；空的 data 数组表示没有匹配项。Service 不是模型名称：同一模型在不同服务下的素材、取消、草稿能力可能不同。</p>
""" + code('json', json.dumps({'object':'list','data':[{'service':'seedance-standard','model':MINI,'material_management':True,'storage_types':['Image','Video','Audio'],'reference_types':['Image','Video','Audio'],'upload':{'enabled':True,'max_bytes':104857600,'expires_seconds':86400,'audio':False},'task_list':True,'task_cancel':True,'task_delete':{'local':True,'upstream':False},'draft':False,'draft_upgrade':False,'mov':False,'frames':False,'automatic_ingestion':False,'real_person_verification':False,'cross_account_copy':False,'verification':'administrator_configured'}],'asset_api_version':'2024-01-01'}, ensure_ascii=False,indent=2)) + table(['服务标识', '当前配置模型', '素材管理 / 引用', '取消生成'], [
 ['<code>seedance-standard</code>', '2.0 mini / 2.0 / 2.0 fast / 2.5', '图片、视频、音频素材库；四个模型均可引用三类素材；签名上传仅图片/视频', '已启用；fast 实测取消成功'],
 ['<code>seedance-alternate</code>', '2.0', '图片、视频、音频素材库；2.0 音频 ID 生成已验证；签名上传仅图片/视频', '未启用'],
 ['<code>seedance-shujubao</code>', '2.0、fast、mini、2.5', '图片、视频、音频素材库；真人认证由独立能力开关控制', '以 capabilities 为准'],
 ['<code>seedance-upload</code>', '2.0 mini', '临时上传流程；无独立素材库；当前上游模型不可用，见兼容说明', '未启用'],
]) + """<p>截至 2026-10-27。下列服务是否启用以实时能力响应为准；均复用任务列表及终态本地隐藏。seedance-standard 的 2.5 draft、draft_upgrade、mov 已配置开放，具体组合的实测范围见兼容说明。verification=administrator_configured 表示管理员配置声明，不表示每个参数组合均已通过实测。storage_types 表示可入库类型，reference_types 表示该模型可用于生成的类型；upload.audio 单独表示本地音频上传。standard 四个模型及 alternate 2.0 允许音频素材 ID 引用，不代表音频文件直传开放。调用方应刷新能力列表，不能将类型固定为 Image/Video。以实时能力响应、令牌分组和模型权限为准；需要开通服务时联系管理员。</p>""")

section('素材与上传', 'asset-mode', '素材库接入流程', '创建素材组 → 创建素材 → 等待 Active → 引用生成。', """
<p>本站已提供独立素材库管理，支持图片、视频素材的创建、查询、更新、删除及分页；seedance-standard、seedance-alternate，以及启用后的 seedance-shujubao 支持音频 URL 入库和同样的管理操作。素材按用户隔离：同一用户的多个 API Key 共享素材，其他用户不可访问。API Key 还须具备所选服务的模型与分组权限。</p>
<ol><li>调用能力查询，选择 material_management=true 的服务。</li><li>创建 AIGC 素材组，保存本站 jdg_ 开头的组 ID。</li><li>通过公网 URL 或已完成上传的 UploadId 创建素材，保存 jda_ 开头的素材 ID。</li><li>轮询 GetAsset，确认 Status=Active 后，用 asset://jda_... 生成。</li></ol>
""" + note('支持 AIGC 素材组；数据宝服务的真人认证另见下方专节，以 real_person_verification 能力为准。不提供跨账号复制或自动入库承诺。历史 image_source_mode=asset / video_source_mode=asset 属于渠道扩展，不能替代本站素材创建流程；不要据此预期任务结果返回可复用的本站素材 ID。') + """<p>旧的裸上游 asset:// ID 不能直接当作本站素材使用，需要通过本站接口重新创建。本站固定素材所属账号，禁止跨账号混用或自动重试到其他账号；账号凭据改变后可能需要重新入库。</p>""")

section('素材与上传', 'asset-api', '素材与素材组 API', '十个 Action 共用一个入口，字段名区分大小写。', """
<div class="endpoint"><span class="method">POST</span><code>/doubao/api/v3/assets/?Action=...&amp;Version=2024-01-01</code></div>
<p>请求头为 Authorization: Bearer &lt;API_KEY&gt; 和 Content-Type: application/json。所有创建操作另需 Idempotency-Key（1–128 字节）；修改、删除、查询不要求此头。Name 最多 64 个字符，Description 最多 256 个字符。</p>
""" + table(['Action', '请求 JSON（示例）', '说明'], [
 ['CreateAssetGroup', '<code>{"Service":"seedance-standard","Model":"'+MINI+'","Name":"项目素材","GroupType":"AIGC"}</code>', 'Service、Name 必填；Model 建议填写以检查模型权限；Description 可选'],
 ['GetAssetGroup', '<code>{"Id":"jdg_REPLACE"}</code>', '查询自己的素材组'],
 ['UpdateAssetGroup', '<code>{"Id":"jdg_REPLACE","Name":"新名称","Description":"新说明"}</code>', '更新名称或说明'],
 ['DeleteAssetGroup', '<code>{"Id":"jdg_REPLACE"}</code>', '删除组，同时使组内素材不可用'],
 ['ListAssetGroups', '<code>{"Service":"seedance-standard","PageNumber":1,"PageSize":20}</code>', '仅列出本站当前用户的素材组'],
 ['CreateAsset', '<code>{"GroupId":"jdg_REPLACE","AssetType":"Image","Name":"参考图","URL":"https://your-domain.example/image.png"}</code>', 'GroupId、AssetType 必填；URL 与 UploadId 二选一；Name 可选；类型为 Image / Video / Audio，以服务 storage_types 为准；Audio 当前仅支持 URL 入库'],
 ['GetAsset', '<code>{"Id":"jda_REPLACE"}</code>', '刷新状态；失败时可含 Error.Code / Error.Message；响应可能含临时预览 URL'],
 ['UpdateAsset', '<code>{"Id":"jda_REPLACE","Name":"新名称"}</code>', '修改名称'],
 ['DeleteAsset', '<code>{"Id":"jda_REPLACE"}</code>', '删除素材，之后不可再引用'],
 ['ListAssets', '<code>{"GroupId":"jdg_REPLACE","Status":"Active","PageNumber":1,"PageSize":20}</code>', '本站归属列表；状态是最后观察值'],
]) + '<h3 id="create-asset-group">1. 创建素材组</h3>' + code('bash', curl(ASSETS.format('CreateAssetGroup'), {'Service':'seedance-standard','Model':MINI,'Name':'项目素材','GroupType':'AIGC'}, extra=' -H "Idempotency-Key: project-001"')) + code('json', json.dumps({'ResponseMetadata':{'RequestId':'REQUEST_ID','Action':'CreateAssetGroup','Version':'2024-01-01'},'Result':{'Id':'jdg_REPLACE','Name':'项目素材','Description':'','Service':'seedance-standard','GroupType':'AIGC','CreateTime':'2026-10-23T00:00:00Z','UpdateTime':'2026-10-23T00:00:00Z'}},ensure_ascii=False,indent=2)) + '<h3 id="create-asset">2. 创建素材并查询状态</h3>' + code('bash', curl(ASSETS.format('CreateAsset'), {'GroupId':'jdg_REPLACE','AssetType':'Image','Name':'参考图','URL':'https://your-domain.example/image.png'}, extra=' -H "Idempotency-Key: image-001"')) + code('bash', curl(ASSETS.format('GetAsset'), {'Id':'jda_REPLACE'})) + code('json', json.dumps({'ResponseMetadata':{'RequestId':'REQUEST_ID','Action':'GetAsset','Version':'2024-01-01'},'Result':{'Id':'jda_REPLACE','GroupId':'jdg_REPLACE','Name':'参考图','Service':'seedance-standard','AssetType':'Image','Status':'Active','Reference':'asset://jda_REPLACE','CreateTime':'2026-10-23T00:00:00Z','UpdateTime':'2026-10-23T00:00:05Z'}},ensure_ascii=False,indent=2)) + """
<p>创建响应可能为 Processing，不能马上生成。通过 GetAsset 刷新至 Active；若 Result.Status=Failed，读取 Result.Error.Code / Result.Error.Message 定位失败原因，详见<a href="#asset-failure">素材查询失败详情</a>。URL 必须是公开可读的 HTTP(S) 地址，端口限 80/443；内网地址、本地路径、需登录页面不可用。不会透传跳过审核的参数。</p>
<h3 id="asset-pagination">分页与响应</h3><p>成功响应统一包在 Result 内，并带 ResponseMetadata。列表支持 PageNumber（或 Page）、PageSize（默认 20，1–200）、NextToken（下一页数字字符串）。素材列表可加 GroupId / GroupIds（最多 100 个）、Status、AssetType、Service；组列表使用 Service 过滤。查询页码为 1–100000。</p>
""" + code('json', json.dumps({'ResponseMetadata':{'RequestId':'REQUEST_ID','Action':'ListAssets','Version':'2024-01-01'},'Result':{'Items':[],'TotalCount':0,'PageNumber':1,'PageSize':20,'NextToken':'','StatusConsistency':'last_observed'}},indent=2)) + """<p>NextToken 为空表示末页。列表状态为缓存观察值，需要新状态时调用 GetAsset；生成前也会校验素材最新状态。列表不会导入或枚举历史共享上游账号中的其他素材。</p>""" + note('相同用户、同类资源、相同 Idempotency-Key 和相同请求可安全重试；同 Key 改参数返回 409。上游创建结果不确定时也会保留待确认状态并返回 409，请联系管理员核对，勿盲目换 Key 重复创建。视频生成接口不因此自动获得素材创建接口的幂等保证。'))

section('素材与上传', 'shujubao', '数据宝服务接入', '使用同一套 JDOTOKEN 素材接口，显式选择数据宝服务。', """
<p>先查询 <code>GET /v1/seedance/capabilities</code>，确认令牌可见 <code>seedance-shujubao</code>。接口示例只适用于已经启用此服务的环境；未返回该服务时不要直接使用示例。模型名使用本站完整名称，不改成 seedance-2.0 等渠道别名。</p>
""" + code('bash', curl(ASSETS.format('CreateAssetGroup'), {'Service':'seedance-shujubao','Model':MINI,'Name':'数据宝项目素材'}, extra=' -H "Idempotency-Key: shujubao-project-001"')) + """
<p>取得本站 <code>jdg_</code> ID 后，使用上方十个素材 / 素材组 Action。图片、视频、音频分别使用 Image、Video、Audio；音频通过公网 URL 入库。GetAsset 返回 Active 后，将本站 <code>asset://jda_...</code> 放入对应媒体字段，生成时可以显式指定 <code>seedance_service: seedance-shujubao</code>。</p>
""" + code('bash', curl(VIDEO, {'seedance_service':'seedance-shujubao','model':MINI,'content':[{'type':'text','text':'让参考图中的玩具机器人轻轻挥手。'},{'type':'image_url','role':'reference_image','image_url':{'url':'asset://jda_REPLACE'}}],'duration':4,'resolution':'480p','generate_audio':False})) + """
<p>视频生成、单任务查询、任务列表及结果下载继续使用本文统一入口。终态任务可在本站隐藏；取消、草稿等能力只在 capabilities 明确启用时使用。数据宝文档的 /v1/api/assets/ 是渠道内部入口，不是本站对外路径。</p>
""" + note('素材绑定原服务和渠道账号，不能将 Lingxi / 移动云的素材 ID 转给数据宝，也不能传入裸上游 asset:// ID。素材创建仍需显式指定自己的素材组和 Idempotency-Key；不启用共享默认组，不透传跳过审核参数。数据宝返回素材状态同步中时，本站响应 409 upstream_resource_busy 和 Retry-After: 2；稍后以相同请求重试。'))

section('素材与上传', 'real-person', '真人认证与素材组', '本人完成活体认证后，将认证结果绑定为当前用户自己的素材组。', """
<p>仅在所选服务返回 <code>real_person_verification: true</code> 时可用。当前适配数据宝；其他服务不会随之自动开放。真人认证不是 JDOTOKEN 登录，也不会改变账户或令牌权限。</p>
<h3 id="real-person-create">1. 创建认证会话</h3><p>CallbackURL 必须是你的公网 HTTPS 回跳地址，端口为 443，不可包含账号密码或 URL 片段。创建必须带 Idempotency-Key，建议指定 Model 以校验目标模型权限。</p>
""" + code('bash', curl(ASSETS.format('CreateVisualValidateSession'), {'Service':'seedance-shujubao','Model':MINI,'CallbackURL':'https://your-domain.example/verification-complete'}, extra=' -H "Idempotency-Key: person-verification-001"')) + code('json', json.dumps({'Result':{'SessionId':'jdv_REPLACE','BytedToken':'PRIVATE_VERIFICATION_TOKEN','H5Link':'https://verification-provider.example/session','CallbackURL':'https://your-domain.example/verification-complete','Service':'seedance-shujubao','ExpiresAt':1793100000}},ensure_ascii=False,indent=2)) + """
<p>让需要授权的本人打开 H5Link，按页面要求确认授权并完成活体认证。BytedToken 和 H5Link 是短期敏感凭证，不要写入公开日志或分享给其他人；会话有效期最多 30 分钟，以 ExpiresAt 为准。相同创建 Key 重试不会重复创建；结果不确定时保留原 Key 并联系管理员核查。</p>
<h3 id="real-person-result">2. 查询认证结果</h3><p>浏览器回跳参数只用于提示；是否认证成功必须通过服务器查询确认。使用创建会话时返回、且属于当前用户的 BytedToken：</p>
""" + code('bash', curl(ASSETS.format('GetVisualValidateResult'), {'BytedToken':'PRIVATE_VERIFICATION_TOKEN'})) + code('json', json.dumps({'Result':{'GroupId':'jdg_REPLACE','GroupType':'LivenessFace','Service':'seedance-shujubao'}},ensure_ascii=False,indent=2)) + """
<p>成功时返回本站 jdg_ 素材组 ID，不暴露上游组 ID；可继续使用 CreateAsset 上传同一人物的素材，等待 Active 后用于生成。不能使用 CreateAssetGroup 直接伪造 LivenessFace 组。组删除后，旧认证结果不能重新创建该组。</p>
<p>409 verification_pending 表示尚未完成；410 verification_expired 表示会话过期，需要新建。404 可能是凭证不属于当前用户，或上游暂未提供认证结果；不能当作认证成功。403 upstream_capability_unavailable 表示渠道账号未开通功能。本人完成认证、真人组入库和成片属于人工验收环节；创建认证链接成功不代表这一整条链路已经通过。</p>
""" + note('不同用户之间不能交换认证凭证；查询仍检查原服务、模型权限及渠道账号。渠道换 Key 或停用后旧会话不能继续导入素材组。'))

section('素材与上传', 'uploads', '上传本地文件', '先申请上传，再 PUT 文件，最后确认并获取引用。', """
<p>使用本站签名上传，不需要自行配置对象存储。支持 PNG、JPEG、WebP 图片和 MP4、MOV 视频；暂不支持本地音频文件上传（upload.audio=false）；音频请通过公开 URL 调用 CreateAsset 入库，见<a href="#audio-reference">音频素材章节</a>。单文件最多 100 MiB（104857600 字节），每用户最多 20 个未过期预留，有效期为申请后的 24 小时。MOV 上传类型不代表所有模型支持 MOV 输出。</p>
<div class="endpoint"><span class="method">POST</span><code>/v1/seedance/uploads/presign</code></div>
""" + code('bash', curl('/v1/seedance/uploads/presign',{'service':'seedance-standard','model':MINI,'content_type':'image/png','size':12345})) + code('json', json.dumps({'id':'jdu_REPLACE','upload_url':'https://www.jdotoken.com/v1/seedance/uploads/jdu_REPLACE/content?expires=EXPIRES&signature=SIGNATURE','method':'PUT','headers':{'Content-Type':'image/png'},'expires_at':1792800000,'max_bytes':12345,'state':'pending','complete_url':'/v1/seedance/uploads/jdu_REPLACE/complete'},indent=2)) + """<p>size 必须为真实文件字节数；响应 max_bytes 等于本次预留长度。使用返回的 upload_url 和 headers 原样 PUT 文件，不能发送 multipart 包装，也无需附带 API Key。签名只能使用一次，重复 PUT 返回 409；格式、文件签名、长度均会校验。</p>
<h3 id="upload-example">完整 Python 示例</h3><p>示例使用本地 reference.png；改为其他格式时同时修改 content_type。所有网络异常都应检查已有会话状态，避免盲目重新提交。</p>
""" + code('python', '''import json
import os
from pathlib import Path
from urllib.request import Request, urlopen

base = "https://www.jdotoken.com"
key = os.environ["JDOTOKEN_API_KEY"]

def post(path, payload):
    req = Request(base + path, data=json.dumps(payload).encode(),
                  headers={"Authorization": "Bearer " + key,
                           "Content-Type": "application/json"}, method="POST")
    with urlopen(req, timeout=120) as response:
        return json.load(response)

file_bytes = Path("reference.png").read_bytes()
session = post("/v1/seedance/uploads/presign", {
    "service": "seedance-standard",
    "model": "doubao-seedance-2-0-mini-260615",
    "content_type": "image/png", "size": len(file_bytes),
})
put = Request(session["upload_url"], data=file_bytes,
              headers=session["headers"], method="PUT")
with urlopen(put, timeout=300) as response:
    uploaded = json.load(response)
complete = post(session["complete_url"], {})
print(complete["id"], complete["source"], complete["expires_at"])
''') + code('json', json.dumps({'id':'jdu_REPLACE','state':'complete','source':'upload://jdu_REPLACE','size':12345,'sha256':'FILE_SHA256','expires_at':1792800000,'material_status':'not_ingested'},indent=2)) + """
<p>确认接口为 POST /v1/seedance/uploads/{id}/complete，使用本站 API Key。完成后可将 source 直接用于视频生成；也可以对支持素材库的服务，使用 CreateAsset 的 UploadId 字段入库（此时不要同时传 URL）。</p>
""" + code('json', json.dumps({'GroupId':'jdg_REPLACE','AssetType':'Image','Name':'参考图','UploadId':'jdu_REPLACE'},ensure_ascii=False,indent=2)) + note('state=complete / material_status=not_ingested 仅表示上传完成，不表示素材已经入库或审核为 Active。upload:// 是临时引用；需要复用素材时另行 CreateAsset 并等待 Active。seedance-upload 服务只能直接引用上传文件，不提供独立素材库管理。') + """<p>签名 URL 属于临时访问凭据，请勿公开。PUT 签名不能用于 GET/HEAD；平台在生成时自动给渠道提供相应下载签名，调用方无需自行计算。签名下载支持 GET、HEAD 和 Range（206），与生成视频下载代理的 HEAD 兼容性是不同接口。</p>""")

section('素材与上传', 'asset-reference', '使用素材或上传引用生成', '使用本站 ID 固定素材所属服务和账号。', code('bash', curl(VIDEO, {'model':MINI,'seedance_service':'seedance-standard','content':[{'type':'text','text':'参考图中的小机器人轻轻挥手，镜头固定。'},{'type':'image_url','role':'reference_image','image_url':{'url':'asset://jda_REPLACE'}}],'duration':4,'resolution':'480p','generate_audio':False})) + """
<p>图片使用 image_url / reference_image，视频使用 video_url / reference_video；standard 四个模型及 alternate 2.0 的音频使用 audio_url / reference_audio。图片、视频引用可为 asset://jda_... 或已完成的 upload://jdu_...；音频目前使用 URL 入库后返回的 asset://jda_...。素材必须归属当前用户、状态 Active、类型受模型支持；临时上传必须尚未过期。</p>
<p>使用 OpenAI JSON 时，把 content 放入 metadata.content；seedance_service 可放顶层以显式选择能力查询返回的服务。它不能绕过令牌权限。素材或上传引用本身也会固定其账号，不允许混用不同账号。本站最多接收 16 个此类引用，模型可能有更严格数量限制。</p>
<p>生成后仍使用本站返回的 task_... ID 查询。任务结果中的上游字段不能替代素材创建返回的 jda_... ID。已有草稿若没有可靠的同账号来源，不能与本站素材混用。</p>
""")

audio_image = {'type':'image_url','role':'reference_image','image_url':{'url':'asset://jda_IMAGE_REPLACE'}}
audio_video = {'type':'video_url','role':'reference_video','video_url':{'url':'asset://jda_VIDEO_REPLACE'}}
audio_reference = {'type':'audio_url','role':'reference_audio','audio_url':{'url':'asset://jda_AUDIO_REPLACE'}}
audio_prompt = '参考图片中机器人的外观与动作，将参考音频作为柔和的背景声，生成自然挥手的视频。'
audio_content = [{'type':'text','text':audio_prompt}, audio_image, audio_reference]
audio_all_content = [{'type':'text','text':audio_prompt}, audio_image, audio_video, audio_reference]
audio_native = {'seedance_service':'seedance-standard','model':MINI,'content':audio_content,'duration':4,'resolution':'480p','ratio':'16:9','generate_audio':True}
audio_openai = {'seedance_service':'seedance-standard','model':MINI,'prompt':audio_prompt,'seconds':'4','size':'854x480','metadata':{'content':audio_content,'duration':4,'resolution':'480p','ratio':'16:9','generate_audio':True}}
full_audio_content = [{'type':'text','text':audio_prompt}] + [{'type':'image_url','role':'reference_image','image_url':{'url':f'asset://jda_IMAGE_{n}_REPLACE'}} for n in range(1,5)] + [{'type':'audio_url','role':'reference_audio','audio_url':{'url':f'asset://jda_AUDIO_{n}_REPLACE'}} for n in range(1,3)] + [audio_video]
full_audio_native = {'seedance_service':'seedance-standard','model':FULL,'content':full_audio_content,'duration':15,'resolution':'720p','ratio':'9:16','generate_audio':True,'image_source_mode':'asset','video_source_mode':'asset'}
full_audio_openai = {**audio_openai,'model':FULL,'metadata':{**audio_openai['metadata'],'content':[{'type':'text','text':audio_prompt},audio_image,*full_audio_content[5:7]]}}
section('素材与上传', 'audio-reference', '使用音频素材生成', '音频 URL 入库后，使用素材 ID 配合图片或视频生成。', """
<p><strong>已验证并开放：seedance-standard 的 mini、2.0、2.0 fast、2.5，以及 seedance-alternate 的 2.0。</strong>2026-10-24 经本站原生接口验证图片＋音频，经 OpenAI 兼容接口验证图片＋视频＋音频：任务均完成，4 秒、480p 成品下载及完整解码通过，包含非静音 AAC 音轨。2026-10-26 已补充完整版 2.0：4 图＋2 音频＋1 视频，15 秒、720p、9:16 原生生成，以及图片＋双音频的 OpenAI 兼容生成均通过本站验证。同日补验 standard fast、2.5 的图片＋视频＋音频素材 ID 生成，以及 alternate 2.0 的图片＋音频素材 ID 生成，均为 4 秒、480p，成品含非静音音轨。纯音频单独生成未验收；seedance-upload 未开放音频素材库。</p>
<p>先查询 capabilities：服务 storage_types 和所选模型 reference_types 均须包含 Audio。音频 URL 入库、音频 ID 生成、本地音频文件上传是三项独立能力；当前本地音频上传仍未开放。</p>
<h3 id="audio-ingestion">1. 提交音频素材</h3><p>先按素材 API 示例在 seedance-standard 或 seedance-alternate 创建素材组。以下 URL 替换为可公开读取的有效 WAV 地址，GroupId 替换为本站 jdg_ ID；本轮已验证 WAV，不据此保证其他音频格式。相同逻辑创建重试保持请求与 Idempotency-Key 不变，新素材使用新的 Key。</p>
""" + code('bash', curl(ASSETS.format('CreateAsset'), {'GroupId':'jdg_REPLACE','AssetType':'Audio','Name':'参考音频','URL':'https://your-domain.example/reference.wav'}, extra=' -H "Idempotency-Key: audio-001"')) + code('bash', curl(ASSETS.format('GetAsset'), {'Id':'jda_AUDIO_REPLACE'})) + """
<p>保存 CreateAsset 返回的 Result.Id。轮询 GetAsset，确认 Result.AssetType=Audio、Result.Status=Active，再使用 asset:// 加上该本站 ID。素材列表可用 AssetType=Audio 过滤；创建、查询、更新、列表、幂等及删除均已验证。</p>
<h3 id="audio-native">2. 原生接口：图片＋音频</h3><p>替换下列图片与音频 ID。它们必须归属当前用户和同一服务账号，并均处于 Active。audio_url 是 content 中的素材项类型；generate_audio=true 请求输出带声音的视频，不能替代音频素材引用。</p>
""" + code('bash', curl(VIDEO, audio_native)) + '<h3 id="audio-native-three">原生接口：图片＋视频＋音频</h3><p>在上述结构中增加 video_url / reference_video。三类素材组合已通过本站 OpenAI 兼容入口验收；此处为对应原生请求写法。</p>' + code('bash', curl(VIDEO, {**audio_native,'content':audio_all_content})) + '<h3 id="audio-openai">3. OpenAI 兼容接口</h3><p>扩展参数放入 metadata，服务和公开模型名称与原生接口相同。下面先给出图片＋音频写法，再给出本轮通过 OpenAI 入口实测的三类素材组合。</p>' + code('bash', curl('/v1/videos', audio_openai)) + code('bash', curl('/v1/videos', {**audio_openai,'metadata':{**audio_openai['metadata'],'content':audio_all_content}})) + """
""" + '<h3 id="audio-full">4. 完整版 2.0：多素材与双音频</h3><p>以下原生结构已用独立合成素材完成本站验证：4 张图片、2 段不同音频、1 段视频，15 秒、720p、9:16。请替换全部 ID，模型保持完整版 2.0，无需改成 mini。实测支持该输入结构，不保证任意人物素材、声音模仿或复杂分镜均能通过上游审核或完全按提示生成。</p>' + code('bash', curl(VIDEO, full_audio_native)) + '<p>完整版 OpenAI 兼容双音频示例（4 秒、480p）：</p>' + code('bash', curl('/v1/videos', full_audio_openai)) + note('reference_video 会作为真实视频参考发送。若视频只是调用方的占位内容，应从 content 删除该项；提示词中的“请勿参考”不会让网关移除它。') + """
""" + '<h3 id="audio-model-matrix">5. fast、2.5 与 alternate 2.0</h3><p>以下三种组合已分别完成本站真实生成、查询、下载和音轨校验。fast 与 2.5 示例均为图片＋视频＋音频，4 秒、480p；alternate 2.0 示例为图片＋音频。请以相应服务创建的新素材 ID 替换示例，不能复用另一个服务的 ID。</p>' + code('bash', curl(VIDEO, {**audio_native,'model':'doubao-seedance-2-0-fast-260128','content':audio_all_content})) + code('bash', curl('/v1/videos', {**audio_openai,'model':'doubao-seedance-2-5-260628','metadata':{**audio_openai['metadata'],'content':audio_all_content}})) + code('bash', curl(ASSETS.format('CreateAssetGroup'), {'Service':'seedance-alternate','Model':FULL,'Name':'音频参考素材组'}, extra=' -H "Idempotency-Key: alternate-audio-project-001"')) + code('bash', curl(VIDEO, {**audio_native,'seedance_service':'seedance-alternate','model':FULL})) + note('seedance-upload 未接入独立素材库；2026-10-26 对应上游账号访问 Ark 素材接口返回 403 manju_gt_not_enabled。同日该渠道四个 Seedance 模型的生成请求均返回 503 model_not_found，当前账号可见模型列表为空，暂不应选择此服务生成。需要上游恢复账号模型及素材权限；本地能力声明或开放 Audio 类型不能解决。') + """
<p>创建成功仅表示受理。保存本站 task_ ID，用原生接口查询至 succeeded，或用 OpenAI 接口查询至 completed，再通过 /v1/videos/{id}/content 下载。含音轨不保证生成音频逐字、逐采样复现参考内容。</p>
""" + note('此前 422 unsupported_reference 的原因是模型 reference_types 未包含 Audio；standard 四个模型及 alternate 2.0 现已开放。若仍出现该错误，核对 service、完整模型 ID 及实时能力响应。若客户端在发送请求前报“仅支持 Image/Video”，应更新调用方本地类型校验。其他错误与重试方式见<a href="#audio-errors">音频排错</a>。'))

openai_video = {'model': MINI, 'prompt':'参考图片中的机器人轻轻挥手，镜头固定。', 'seconds':4, 'metadata':{'resolution':'480p','ratio':'16:9','draft':False,'generate_audio':False,'watermark':False,'return_last_frame':True,'execution_expires_after':3600,'content':[{'type':'image_url','role':'reference_image','image_url':{'url':'https://your-domain.example/character.jpg'}}]}}
section('视频 API', 'openai-video', 'OpenAI 视频接口', '使用 JSON 创建任务，Seedance 扩展参数放进 metadata。', '<div class="endpoint"><span class="method">POST</span><code>/v1/videos</code></div>' + code('bash', curl('/v1/videos', openai_video)) + note('已确认顶层 <code>ratio</code> 和 <code>return_last_frame</code> 会被忽略。请像示例一样把分辨率、比例、音频、水印、尾帧和素材模式放入 <code>metadata</code>。本地 multipart 文件直传尚未开放；可先通过签名上传获得 upload:// 引用。metadata.content 也可使用本站 asset:// 素材、HTTPS URL 或图片 Base64 data URL。音频素材 ID 的两种组合见<a href="#audio-openai">音频 OpenAI 示例</a>。') + '''
<p>上例为已验收的请求结构，请替换图片 URL。仅文生视频时移除 metadata.content，保留 prompt。通用任务创建也可用相同 JSON 提交至 POST /v1/tasks/doubao。</p>
<h3 id="openai-video-query">查询 OpenAI 视频状态</h3>
''' + code('bash', curl('/v1/videos/$JDOTOKEN_TASK_ID')) + '''
<p>此接口成功终态为 completed，生成中为 in_progress，与原生 v3 的 succeeded、running 不同。需要 usage、上游 ID、素材失败细节、尾帧或实际生成参数时，请用同一本站任务 ID 查询原生 v3 接口。</p>
''' + table(['查询路径','响应特点'], [['<code>/doubao/api/v3/contents/generations/tasks/{id}</code>','完整原生任务对象；失败时可含 failure_detail，成功时含 content、usage 和 upstream_task_id。'],['<code>/v1/videos/{id}</code>','简化视频状态；未返回 failure_detail、usage 与 upstream_task_id。'],['<code>/v1/video/generations/{id}</code>','旧版查询可用，状态为 data.status=SUCCESS/FAILURE；原生对象在 data.data，失败详情在 data.data.failure_detail。缺少 SDGO 的同层 timing 等字段。'],['<code>/v1/tasks/{id}</code>','通用任务状态、进度、失败原因；产物使用 /artifacts 查询。']]) + '<p><strong>旧版 POST /v1/video/generations 当前不可用</strong>，会返回渠道选择错误；不要因旧版 GET 可以查询而推断旧版创建也可用。</p>')

section('视频 API', 'polling', '查询任务与结果', '使用创建时保存的本站任务 ID。', '<div class="endpoint"><span class="method get">GET</span><code>' + VIDEO + '/{task_id}</code></div>' + code('bash', 'export JDOTOKEN_TASK_ID="task_替换为创建响应中的ID"\n' + curl(VIDEO+'/$JDOTOKEN_TASK_ID')) + '''
<p>建议每隔 5–10 秒查询一次；若限流，延长间隔。下方脚本只查询已有任务，最长等待 10 分钟，不会再次创建视频。</p>
''' + code('python', '''import json
import os
import time
from urllib.error import HTTPError, URLError
from urllib.parse import quote
from urllib.request import Request, urlopen

task_id = os.environ["JDOTOKEN_TASK_ID"]
url = "https://www.jdotoken.com/doubao/api/v3/contents/generations/tasks/" + quote(task_id, safe="")
headers = {"Authorization": "Bearer " + os.environ["JDOTOKEN_API_KEY"]}
deadline = time.monotonic() + 600
while time.monotonic() < deadline:
    try:
        with urlopen(Request(url, headers=headers), timeout=30) as response:
            task = json.load(response)
    except HTTPError as error:
        if error.code not in (429, 500, 502, 503, 504):
            raise
        time.sleep(10)
        continue
    except (URLError, TimeoutError):
        time.sleep(10)
        continue
    status = task.get("status")
    print(status)
    if status == "succeeded":
        print(task.get("content", {}).get("video_url", "请使用本站视频下载接口"))
        break
    if status in ("failed", "expired", "cancelled", "canceled"):
        raise RuntimeError(json.dumps(task, ensure_ascii=False))
    time.sleep(5)
else:
    raise TimeoutError("停止本地等待；任务可能仍在运行，请保存 task_id 后继续查询。")
''') + table(['状态', '处理方式'], [['queued','排队中，继续等待。'],['running','生成中，继续查询。'],['succeeded','已成功，读取视频地址或通过本站下载。'],['failed / expired','终止本次轮询，查看 error / failure_detail 或控制台失败原因。'],['其他状态','保留原始响应；不要将未知状态视为成功。上游取消状态也应停止轮询。']]) + '''
<h3 id="video-result">成功结果示例</h3><p>依据 2.0 mini 实测结构整理，任务 ID 与视频地址已替换。响应可能包含更多字段。</p>
''' + code('json', json.dumps({'id':'task_YOUR_TASK_ID','status':'succeeded','model':MINI,'content':{'video_url':'https://media.example/result.mp4?temporary-signature=...'},'duration':4,'resolution':'480p','ratio':'16:9','draft':False,'framespersecond':24,'usage':{'completion_tokens':40594,'total_tokens':40594}}, indent=2)) )

section('视频 API', 'download', '下载视频与产物', '及时保存生成结果。', '''
<div class="endpoint"><span class="method get">GET</span><code>/v1/videos/{task_id}/content</code></div>
<p>任务成功后使用此接口下载 MP4。<code>content.video_url</code> 也可能提供临时签名地址，该地址有有效期，应及时下载到自己的存储。</p>
''' + code('bash', curl('/v1/videos/$JDOTOKEN_TASK_ID/content', extra=' --output result.mp4')) + '''
<p>GET 下载已验证；Range 请求可返回 206。当前不要依赖 HEAD 探测下载是否可用：部分签名媒体的 HEAD 会返回 502，但 GET 正常。</p>
<p>开启 return_last_frame 后，尾帧地址见 content.last_frame_url。实际返回可能是 JPEG；产物列表当前可能误标 image/png，请以下载响应 Content-Type 和文件字节为准。</p>
<h3 id="artifacts">查询产物列表</h3>
''' + code('bash', curl('/v1/tasks/$JDOTOKEN_TASK_ID/artifacts')) + '''
<p>产物列表成功时返回 video，以及请求了尾帧时的 last_frame。content_url 是临时签名链接，可直接获取，需按敏感链接保管。也可用 Bearer Key 请求 /v1/tasks/{task_id}/artifacts/last_frame/content。本站下载代理依赖源文件可用性，不代表永久保存。</p>
''')

section('视频 API', 'draft', '草稿预览', '仅 Seedance 2.5 支持草稿模式。', '''
<p><strong>mini 不支持草稿或草稿转正片。</strong>mini 的 draft=true 不受支持（历史插件返回 400，服务能力校验可返回 422）；不要将此模型限制误认为路径不支持。2.5 已实测通过 <code>draft=true</code>、<code>resolution=480p</code>、4 秒的草稿生成，并可正常查询和下载。草稿仍会计费。</p>
''' + code('bash', curl(VIDEO, {**video, 'model':'doubao-seedance-2-5-260628','draft':True})) + '''
<p>使用与普通视频相同的查询和下载流程。草稿成功不表示正式成片会自动生成；草稿转正式视频属于另外的任务链路；当前已有参数兼容失败记录，尚无本站完整成功验收，不建议用于生产流程。不要在 mini 请求中引用 2.5 草稿 ID，否则会返回模型不匹配。</p>
''')

section('视频 API', 'task-management', '任务列表、取消与隐藏', '列表只返回当前用户的 Seedance / Doubao 任务。', table(['请求', '用途'], [[f'GET {VIDEO}', '原生 v3 任务列表'],['GET /v1/videos','OpenAI 路径列表；使用页码分页'],[f'DELETE {VIDEO}/{{task_id}}','非终态申请取消；终态在本站隐藏'],['DELETE /v1/videos/{task_id}','与原生 DELETE 相同语义']]) + code('bash', curl(VIDEO+'?page=1&page_size=20&filter.status=succeeded')) + """
<p>page 默认为 1（范围 1–100000），page_size 默认为 20（范围 1–200）。可用筛选：filter.task_ids（重复参数或逗号分隔，最多 100 个）、filter.model、filter.status、filter.service_tier。filter.status 使用 queued、running、succeeded、failed、cancelled、expired。按任务创建顺序排列（较早任务在前）。</p>
""" + code('json', json.dumps({'items':[],'total':0,'page':1,'page_size':20},indent=2)) + """<p>上例是原生列表响应。OpenAI 路径返回 object=list、data、has_more、total；使用相同 page/page_size 参数，不代表完整实现 OpenAI SDK 的游标分页契约。不是其他视频厂商的通用任务列表。</p>
<h3 id="cancel-task">取消未完成任务</h3>
""" + code('bash', curl(VIDEO+'/$JDOTOKEN_TASK_ID',extra=' -X DELETE')) + table(['HTTP / 返回值', '含义'], [['200 / status=cancelled, deleted=false','确认已取消；通过原有任务结算流程处理。'],['202 / status=cancel_requested, deleted=false','已申请，尚未确认；继续查询任务。'],['409 / task_already_terminal','取消确认前任务已经结束；读取 task 或重新查询，不宣称已退款。'],['422 / 能力未启用','当前服务或模型不开放取消。'],['409 / 历史来源不足','旧任务缺少可靠账号来源，不能安全取消；仍可查询。']]) + """
<p>仅能力查询 task_cancel=true 的服务允许取消，且任务需要保存可校验的提交账号来源。断开客户端不会取消任务；上游接受 DELETE 不等于确认取消。当前 seedance-standard 已启用，本轮 fast 实测确认取消并完成退款核对。</p>
<h3 id="hide-task">隐藏已经结束的任务</h3><p>对成功、失败等终态任务发送同一个 DELETE 请求，响应如下。隐藏后本站单任务查询返回 404，列表不再显示。</p>
""" + code('json', json.dumps({'id':'task_YOUR_TASK_ID','deleted':True,'upstream_deleted':False,'billing_records_retained':True},indent=2)) + note('这是本站隐藏记录与结果，不是上游物理文件删除。消费日志、账单和审计保留，隐藏操作不会退还已结算费用。GET /v1/videos/{id}/provenance 尚未开放；请用原生 usage 和控制台消费日志对账。'))

section('用量与排错', 'failure-details', '素材失败与错误详情', '使用原生 v3 查询定位具体阶段。', '''
<h3 id="asset-failure">素材入库失败：GetAsset</h3>
<p>素材查询 HTTP 200 仅表示查询成功。图片、视频、音频均需检查 Result.Status：Processing 继续退避轮询，Active 才可引用，Failed 时读取 Result.Error.Code 和 Result.Error.Message，不应继续等待审核成功或盲目重复提交相同素材。</p>
''' + code('json', json.dumps({'ResponseMetadata':{'RequestId':'REQUEST_ID','Action':'GetAsset','Version':'2024-01-01'},'Result':{'Id':'jda_REPLACE','GroupId':'jdg_REPLACE','AssetType':'Image','Status':'Failed','Reference':'asset://jda_REPLACE','Error':{'Code':'InputImageSensitiveContentDetected.PolicyViolation','Message':'The request failed because the input image may be related to copyright restrictions.'}}},ensure_ascii=False,indent=2)) + '''
<p>以上为失败响应的关键字段示例：上游认为图片可能涉及版权限制。Error 只保留上游提供的非空字符串 Code / Message，并屏蔽当前渠道密钥的直接回显；不改写为确定的侵权结论。错误信息可能为英文，客户端应优先依据 Code 分类，安全地按纯文本展示 Message。</p>
<p>Error、Code、Message 均为可选字段：上游未提供有效原因时不编造；Active / Processing 不返回 Error。此规则适用于 Lingxi、移动云和数据宝的统一 GetAsset，实际原因由各上游提供。ListAssets 为本地缓存列表，不含这次查询的 Error，失败详情请单独调用 GetAsset。请求本身失败时仍使用原有非 2xx 和 error.code/message，勿与 Result.Error 混淆。</p>
<h3 id="video-failure">视频生成任务失败</h3>
<p>下方依据无效图片实测响应整理。HTTP 200 的任务查询仍可能返回 status=failed。检查 error 和 failure_detail，而不是只检查 HTTP 状态。</p>
''' + code('json', json.dumps({'id':'task_YOUR_TASK_ID','status':'failed','error':{'code':'task_failed','message':'第 1 张参考图片格式、内容或大小不符合要求。'},'failure_detail':{'code':'material_invalid','stage':'material_validation','provider_code':None,'materials':[{'type':'image','index':1,'content_index':None,'index_scope':'normalized_sources'}],'message':'请更换有效参考图片后重试。'}},ensure_ascii=False,indent=2)) + '''
<p>material_validation 表示素材校验，material_preparation 表示素材准备。index 按 index_scope 表示素材序号，不能直接当作 content 数组下标；provider_code 或 content_index 可能为空。上游未提供的信息不会凭空补全。</p>
<p>OpenAI 查询目前只保留 error.code/message，丢失结构化详情；旧版查询在 data.data.failure_detail 中提供。排错时优先查询原生 v3，不要把“没有返回详情”理解为“没有具体原因”。</p>
''')

section('用量与排错', 'billing', '价格与用量', '以本站账户的价格和消费记录为准。', '''
<p>查看<a href="/pricing">模型广场</a>中的价格和分组信息，并在<a href="/usage-logs">消费日志</a>中核对实际调用。最终价格取决于模型、分组、输入输出用量及相关参数，供应商对外价格不是本站售价。</p>
<h3 id="reservation">视频任务的预扣与结算</h3>
<p>创建任务时会按规则预扣额度，完成后按实际用量结算并调整差额。预扣金额不是最终费用；2026-10-23 三条成功视频按实际用量核对一致，fast 取消退款核对通过；历史素材失败样本也有预扣退回记录。其他失败或超时情形仍应核对任务终态与实际退款记录。</p>
<p><code>usage.completion_tokens</code> 和 <code>total_tokens</code> 描述模型用量，不直接等于余额或人民币金额。不同模型与分组的倍率可能不同，不要用一个固定换算系数计算所有请求。</p>
''' + note('创建请求超时不代表未创建任务。如果已拿到任务 ID，继续查询该 ID；没有拿到 ID 时先查控制台任务记录，避免重复提交产生多笔消费。'))

section('用量与排错', 'errors', '错误处理与限制', '先确认错误发生在哪个环节。', table(['现象 / HTTP 状态', '建议检查'], [
    ['400 / 参数错误','核对模型、JSON 类型、时长、分辨率和参考素材格式；读取响应错误信息。'],
    ['401 / 鉴权失败','确认使用本站密钥、请求头格式正确，令牌未撤销或过期。'],
    ['403 / 权限不足','检查用户状态、令牌权限、分组或访问限制。'],
    ['404 / 模型、任务或素材不存在','检查路径、本站 ID 和当前用户归属；隐藏后的任务返回 404。'],
    ['409 / 资源冲突','检查 idempotency_conflict、resource_busy、account_changed 或上传状态；不要盲目换幂等 Key 重试创建。'],
    ['413 / 请求过大','新增素材/上传元数据 JSON 上限 1 MiB；文件走签名 PUT，最多 100 MiB 且必须匹配申请字节数。'],
    ['422 / 能力或素材类型不匹配','检查 unsupported_reference、unsupported_asset_type、reference_type_mismatch；分别核对 reference_types、storage_types 和素材字段类型。'],
    ['429 / 限流或可用额度问题','读取具体错误；核对余额与限制。只读轮询可退避重试，创建任务需先排除重复执行。'],
    ['5xx / 服务或上游错误','保留时间、模型、任务 ID 和脱敏错误，查询控制台；不要无条件重复生成。'],
    ['任务状态 failed / expired','HTTP 查询可能成功，但任务业务上已失败。读取 error、failure_detail 或失败原因。'],
]) + '''<p>新增素材与上传接口的错误通常为 {"error":{"code":"错误码","message":"说明"}}；错误语言可能随请求设置变化，请优先依据 code 判断。以上是排查方向，不保证所有上游错误使用相同 HTTP 状态。限流、上下文长度、文件大小、并发和素材规则取决于账户、模型及上游，不存在适用于所有模型的统一数值。</p>
<h3 id="audio-errors">音频与素材引用排错</h3>
''' + table(['错误码 / 表现','处理方式'], [
 ['422 unsupported_reference', '所选模型未开放该素材类型；确认 standard 四个模型或 alternate 2.0 的完整模型 ID，刷新 capabilities；素材须属于所选服务。'],
 ['422 reference_type_mismatch', '音频 ID 必须放在 audio_url 字段；不要放在 image_url 或 video_url。'],
 ['409 asset_not_active / asset_not_ready', '生成前调用 GetAsset，等待素材创建确认且 Status=Active。'],
 ['409 reference_account_conflict / account_changed', '核对素材与所选服务、账号；同一请求的素材必须属于同一服务账号，凭据变动后可能需要重新入库。'],
 ['400 invalid_upload（音频预签名）', '本地音频文件直传尚未开放；改用可公开读取的音频 URL 创建素材。'],
 ['客户端提示“仅支持 Image/Video”', '若请求尚未到达本站，更新调用方类型校验和能力缓存；Audio 入库与引用分开判断。'],
]) + '<h3 id="material-retry">素材并发与重试</h3><p>409 resource_busy 表示资源仍在处理，不等于审核失败。服务器会短暂等待资源锁，超时仍可能返回该错误及 Retry-After: 2。按响应头等待至少 2 秒并增加随机退避后，重试相同请求；创建操作保持原 Idempotency-Key。409 idempotency_conflict 应核对同 Key 的参数是否改变；creation_unconfirmed 表示上游创建结果待核对，不能当作 resource_busy 自动重复提交或换 Key。</p>')

section('用量与排错', 'compatibility', '兼容范围与已知限制', '截至 2026-10-27：区分已实测、配置声明和未开放能力。', table(['能力', '状态 / 验证范围', '接入建议'], [
 ['素材与素材组十个 Action', '两个素材服务全部通过生产验证', '只管理本站当前用户素材，使用 jdg_ / jda_ ID。'],
 ['素材引用生成', 'standard：mini 图片+视频；alternate：2.0 图片，均已生成并下载', '4 秒、480p 的样本，不代表所有参数组合。'],
 ['签名上传 / 上传引用', 'PUT、确认、下载、HEAD、Range、权限及重复上传验证通过；upload 服务 mini 生成成功', '使用 upload:// 引用或另行入库；24 小时有效。'],
 ['任务列表 / 筛选 / 隐藏', '已通过生产验证', '使用原生 v3 或 /v1/videos；page/page_size 分页。'],
 ['取消任务', 'standard fast 实测确认取消、结算与退款核对通过', '其他配置支持模型以实时 task_cancel 为准；alternate / upload 未启用。'],
 ['原生 v3 / OpenAI JSON 创建与查询', '三类服务真实视频验收通过', '扩展参数放 metadata；素材和上传引用可固定服务。'],
 ['图片 URL、Base64 首帧、参考视频、历史 asset 模式', 'mini 历史样本已验证', '不代表自动生成或返回本站素材 ID。'],
 ['音频 URL 入库 / 素材 ID 引用', 'standard 四个模型均通过音频组合；alternate 2.0 图片＋音频通过。标准 2.0 另通过 15 秒/720p/4 图＋2 音频＋1 视频', '先入库并等待 Active；使用 audio_url / reference_audio；素材不可跨服务混用。'],
 ['seedance-upload 当前可用性', '2026-10-26：对应渠道四个 Seedance 模型均 503 model_not_found；素材接口 403 manju_gt_not_enabled', '需上游恢复账号权限；历史成功样本不代表当前可用。'],
 ['移动云图片的 UploadId 入库', '2026-10-27：一个 PNG 样本通过签名上传入库持续 Processing；同文件公网 URL 入库 Active，签名源读取与原文件一致', '当前可优先采用公网图片 URL；等待 GetAsset=Active 后生成，不将创建成功视作审核完成。此现象未在本轮 Lingxi / 数据宝样本复现。'],
 ['取消请求被上游拒绝', '2026-10-27：Lingxi fast 一次取消返回 502 upstream_rejected，任务随后成功；拒绝取消不等于生成失败', '继续查询任务；仅上游确认失败或取消后才退款，成功任务正常结算。不要根据 DELETE 的错误重复创建视频。'],
 ['本地 multipart 文件直传 / 本地音频签名上传', '尚未开放', '图片/视频文件先走签名上传；音频使用 URL 入库，upload.audio=false。'],
 ['数据宝四模型＋三类素材 ID', '2026-10-27 正式环境：2.0 / fast / mini / 2.5 均成功生成；验收组合为三类素材 ID 引用', 'fast 使用 OpenAI 兼容提交，其余使用原生提交；全部 4 秒 / 480p 档，Image＋Video＋Audio。正式开放以 capabilities 为准。'],
 ['真人库认证', '仅在服务声明 real_person_verification=true 时开放', '创建会话、查询结果并取得本站真人组；本人活体认证须手动完成。'],
 ['自动入库 / 跨账号复制 / frames', '当前未开放', '不承诺与任何渠道全部扩展接口等价。'],
 ['2.5 草稿 / 草稿转正片 / MOV 输出', '能力已配置；草稿有历史成功样本，转正片未完成本站成功验收', '先查能力；本轮未重复验收这些生成组合。'],
 ['mini 草稿 / 1080p', '模型限制', '使用普通视频和 480p / 720p。'],
 ['OpenAI 顶层 ratio / return_last_frame', '历史验证会被忽略', '放入 metadata；以实际产物校验参数。'],
 ['OpenAI 完整 usage / failure_detail', '与原生响应不完全等价', '需要完整详情时使用原生 v3 查询。'],
 ['生成视频 HEAD / 尾帧 MIME 标注', '历史存在兼容问题；此次未宣称已修复', '下载用 GET；检查实际图片类型。本站上传签名 HEAD 已验证正常。'],
 ['旧版 /v1/video/generations', '保留兼容入口；本轮未重新完成创建验收', '新接入使用原生 v3 或 /v1/videos；新增列表/取消不使用旧版路径。'],
 ['provenance 对账 API / 生成回调', '尚未开放或未完成验收', '轮询任务，使用控制台消费记录对账。'],
]) + '<p>素材权限隔离、幂等、上传重放拦截及取消结算均已验证。表中服务范围以能力查询为准；管理员配置声明不等于所有模型、素材与参数组合均已实测。</p>')

section('用量与排错', 'changelog', '文档更新', '记录接口说明与验证范围的变化。', '''
<h3 id="release-20261027-shujubao">2026-10-27 · 数据宝适配说明</h3><ul><li>新增数据宝三类素材、四模型生成、真人认证会话与结果查询的接入说明。</li><li>数据宝服务已在正式环境开放；四模型三类素材 ID 生成通过，实际权限以实时能力响应为准。</li><li>明确本人活体认证待人工验收，以及不跨渠道混用素材的限制。</li></ul><h3 id="release-20261026-audio-matrix">2026-10-26 · 音频模型覆盖检查</h3><ul><li>补齐 standard fast、2.5 音频引用，以及 alternate 2.0 音频入库和引用。</li><li>新增三种通过本站实际成片验收的请求示例；保留素材账号隔离和本地音频直传限制。</li><li>说明 seedance-upload 上游账号素材权限阻断，未将受理或配置声明当作成功验收。</li></ul><h3 id="release-20261026-audio">2026-10-26 · 完整版 2.0 音频素材引用</h3><ul><li>standard 的完整版 2.0 新增 Audio 引用，mini 保持支持。</li><li>补充原生 15 秒、720p、9:16、多图片与双音频组合，以及 OpenAI 双音频示例。</li><li>更新模型能力、422 排查和已知限制；说明占位视频仍会作为参考发送。</li></ul><h3 id="release-20261024-audio">2026-10-24 · 音频素材入库与 mini 引用生成</h3><ul><li>更新 standard 的音频入库、mini 音频素材 ID 引用能力和实时能力响应示例。</li><li>新增 WAV URL 入库、图片＋音频及图片＋视频＋音频的原生/OpenAI 请求写法，标明实际验收入口。</li><li>补充 422 类型限制、Active 状态、跨服务引用与客户端类型缓存排查；保留本地音频直传未开放的限制。</li><li>补充 resource_busy 的 Retry-After 与同幂等 Key 重试说明。</li></ul><h3 id="release-20261023">2026-10-23 · 素材管理第一阶段上线</h3><ul><li>新增服务能力查询、素材与素材组十个 Action、幂等与用户归属说明。</li><li>新增本地签名上传、upload:// 与 asset:// 引用生成示例。</li><li>更新任务分页、筛选、取消与终态隐藏语义；移除这些接口尚未开放的旧结论。</li><li>按本轮生产回归更新各服务验证范围，区分配置声明和端到端实测。</li></ul><h3 id="release-20261012-verified">2026-10-12 · 实测更新</h3><ul><li>补充 mini OpenAI JSON、图片 / 视频 asset、Base64 首帧、尾帧和失败详情说明。</li><li>标注旧版创建、上传、列表、取消和对账的明确限制，新增协议响应差异。</li><li>修正下载命令，说明 metadata 参数写法、HEAD 和 MIME 兼容问题。</li><li>更新官方 Logo、公安备案信息与文档布局。</li></ul><h3 id="release-20261012">2026-10-12 · 首版</h3><ul><li>新增快速开始、鉴权、模型目录、对话与流式接入说明。</li><li>新增 Seedance 创建、图片参考、查询、下载与草稿示例。</li><li>根据真实任务记录标注 2.0 mini 与 2.5 的验证范围。</li><li>说明计费、重复提交风险及尚未开放的扩展能力。</li></ul>
<p>最后核对日期：2026-10-27。模型与价格可能先于文档更新，以账户实时列表和控制台为准。</p>
''')
