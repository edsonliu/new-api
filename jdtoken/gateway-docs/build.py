"""Build a dependency-free static documentation release: python3 build.py."""
from html import escape
from pathlib import Path
import hashlib
import json
import shutil

from content import SECTIONS, EXAMPLES

ROOT = Path(__file__).resolve().parent
OUT = ROOT / 'dist'
if OUT.exists():
    shutil.rmtree(OUT)
OUT.mkdir(exist_ok=True)
(OUT / 'assets').mkdir(exist_ok=True)
assets = {}
for filename in ['style.css', 'app.js', 'mark.png', 'logo-horizontal.png', 'police-beian.png']:
    source = ROOT / 'assets' / filename
    digest = hashlib.sha256(source.read_bytes()).hexdigest()[:12]
    name = f'{source.stem}.{digest}{source.suffix}'
    shutil.copyfile(source, OUT / 'assets' / name)
    assets[filename] = '/docs/assets/' + name

navigation = []
group = None
articles = []
for item in SECTIONS:
    if item['group'] != group:
        if group is not None:
            navigation.append('</ul>')
        group = item['group']
        navigation.append(f'<p class="nav-group">{escape(group)}</p><ul>')
    navigation.append(f'<li><a href="#{item["key"]}">{escape(item["title"])}</a></li>')
    articles.append(f'<section id="{item["key"]}" data-title="{escape(item["title"])}"><div class="section-kicker">{escape(item["group"])}</div><h2><a href="#{item["key"]}">{escape(item["title"])}</a></h2><p class="lead">{escape(item["description"])}</p>{item["body"]}</section>')
navigation.append('</ul>')

html = f'''<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>接入文档 · JDOTOKEN</title><meta name="description" content="JDOTOKEN 网关接入指南：API 密钥、对话与流式输出、Seedance 视频生成、素材库、上传、任务查询与取消。提供 cURL、Python 和 JavaScript 示例。">
<link rel="canonical" href="https://www.jdotoken.com/docs/"><link rel="icon" href="{assets['mark.png']}">
<link rel="stylesheet" href="{assets['style.css']}"><script defer src="{assets['app.js']}"></script></head>
<body><a class="skip-link" href="#main">跳转到正文</a>
<header class="topbar"><div class="brand-line"><a class="brand" href="/" aria-label="JDOTOKEN 首页"><img src="{assets['logo-horizontal.png']}" width="170" height="24" alt="及第智算"><span class="product-name">JDOTOKEN</span></a><span class="brand-divider"></span><a class="doc-label" href="/docs/">开发者文档</a></div>
<div class="top-actions"><button id="search-open" class="search-trigger" type="button"><span aria-hidden="true">⌕</span> 搜索文档 <kbd>Ctrl K</kbd></button><a class="console-link" href="/dashboard">控制台 <span aria-hidden="true">↗</span></a><button class="menu-button" id="menu-button" type="button" aria-expanded="false" aria-controls="sidebar">目录</button></div></header>
<div class="layout"><aside class="sidebar" id="sidebar"><p class="sidebar-title">开发者文档</p><nav aria-label="文档目录">{''.join(navigation)}</nav><div class="sidebar-bottom"><span class="status-dot"></span> 接入指南 · 中文版<br><small>更新于 2026.10.27</small></div></aside>
<main id="main"><div class="page-intro"><nav class="breadcrumb" aria-label="面包屑"><a href="/">首页</a><span aria-hidden="true">›</span><span>接入文档</span></nav><div class="eyebrow">JDOTOKEN API</div><h1>模型与视频 API 接入文档</h1><p>从鉴权、请求参数到素材管理、上传与任务管理。使用可复制的示例，接入对话模型与 Seedance 视频生成。</p><button id="intro-search" class="intro-search" type="button"><span aria-hidden="true">⌕</span> 搜索文档、接口或参数… <kbd>Ctrl K</kbd></button><div class="verification">实测更新 · 2026-10-27 <a href="#compatibility">查看支持范围与已知限制 →</a></div></div>{''.join(articles)}
<footer><a class="back-top" href="#main">返回顶部 ↑</a><p>© 2026 JDOTOKEN · 及第嘉序(北京)智算科技有限公司</p><div class="filings"><a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer">京ICP备2026065372号-1</a><a href="https://beian.mps.gov.cn/#/query/webSearch?code=11010602203425" target="_blank" rel="noopener noreferrer"><img src="{assets['police-beian.png']}" width="18" height="20" alt="">京公网安备11010602203425号</a></div></footer></main>
<aside class="on-page"><p>阅读导航</p><nav aria-label="当前章节"><a href="#overview">接入概览</a><a href="#quickstart">快速开始</a><a href="#chat">对话 API</a><a href="#video-workflow">视频 API</a><a href="#openai-video">OpenAI 视频接口</a><a href="#capabilities">服务能力</a><a href="#asset-mode">素材入库</a><a href="#audio-reference">音频素材</a><a href="#uploads">文件上传</a><a href="#task-management">列表与取消</a><a href="#failure-details">失败详情</a><a href="#compatibility">兼容范围</a><a href="#billing">价格与用量</a></nav><div class="help"><span>先从只读请求开始</span><p>查询模型列表，确认密钥与接入地址。</p><a href="#first-request">GET /v1/models ↗</a></div></aside></div>
<dialog id="search-dialog" aria-labelledby="search-title"><div class="search-heading"><h2 id="search-title">搜索文档</h2><button id="search-close" type="button" aria-label="关闭搜索">关闭 Esc</button></div><label for="search-input" class="sr-only">搜索关键词</label><input id="search-input" type="search" placeholder="模型、视频、鉴权、Python…" autocomplete="off"><p id="search-count" aria-live="polite"></p><ul id="search-results"></ul></dialog>
<div id="toast" role="status" aria-live="polite"></div></body></html>'''
(OUT / 'index.html').write_text(html)
(OUT / 'release.json').write_text(json.dumps({'version':'2026.10.28.2','sections':len(SECTIONS),'assets':assets}, ensure_ascii=False, indent=2))
(ROOT / 'examples.json').write_text(json.dumps(EXAMPLES, ensure_ascii=False, indent=2))
print(f'Built {len(SECTIONS)} sections and {len(EXAMPLES)} examples in {OUT}')
