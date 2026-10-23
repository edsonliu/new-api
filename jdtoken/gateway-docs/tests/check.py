"""Validate documentation and interactions without sending model requests."""
import argparse
import ast
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
from playwright.sync_api import sync_playwright, expect

parser = argparse.ArgumentParser()
parser.add_argument('--url', default='http://127.0.0.1:4182/docs/')
parser.add_argument('--output', default='/tmp/jdtoken-docs-check')
args = parser.parse_args()
root = Path(__file__).resolve().parents[1]
out = Path(args.output)
out.mkdir(parents=True, exist_ok=True)
examples = json.loads((root / 'examples.json').read_text())
for number, example in enumerate(examples):
    source = example['source']
    if example['language'] == 'python':
        ast.parse(source)
    elif example['language'] == 'json':
        json.loads(source)
    elif example['language'] == 'javascript':
        with tempfile.NamedTemporaryFile(suffix='.mjs', mode='w') as f:
            f.write(source); f.flush()
            subprocess.run(['node', '--check', f.name], check=True, capture_output=True)
    elif example['language'] == 'bash':
        subprocess.run(['bash', '-n'], input=source, text=True, check=True, capture_output=True)
        if 'curl ' in source:
            # Replace curl with a function to inspect actual arguments, never execute it.
            script = 'curl() { printf "%s\\0" "$@"; }; export JDOTOKEN_API_KEY=TEST_ONLY JDOTOKEN_TASK_ID=task_TEST;\n' + source
            captured = subprocess.run(['bash', '-c', script], check=True, capture_output=True).stdout.decode().split('\0')
            assert any(arg.startswith('https://www.jdotoken.com/') for arg in captured), number
            assert not any(arg == '+' or arg.startswith('+  ') for arg in captured), (number, captured)
            if '-d' in captured:
                payload = json.loads(captured[captured.index('-d') + 1])
                assert isinstance(payload, dict), number

with sync_playwright() as p:
    browser = p.chromium.launch(executable_path=shutil.which('google-chrome') or None, headless=True, args=['--no-sandbox'] + (['--host-resolver-rules=MAP www.jdotoken.com 115.29.196.54'] if args.url.startswith('https://www.jdotoken.com/') else []))
    context = browser.new_context(viewport={'width':1440,'height':1000}, locale='zh-CN', reduced_motion='reduce', permissions=['clipboard-read','clipboard-write'])
    page = context.new_page()
    errors, failures, requests = [], [], []
    page.on('pageerror', lambda e: errors.append(str(e)))
    page.on('response', lambda r: failures.append({'url':r.url,'status':r.status}) if r.status >= 400 else None)
    page.on('request', lambda r: requests.append({'url':r.url,'method':r.method}))
    page.goto(args.url, wait_until='networkidle')
    expect(page).to_have_title('接入文档 · JDOTOKEN')
    assert 'jdtoken' not in page.locator('body').inner_text().lower()
    assert page.locator('.brand img').evaluate('(i)=> Math.abs(i.width/i.height-i.naturalWidth/i.naturalHeight)<0.1')
    section_count = page.locator('main section').count()
    assert section_count == json.loads((root/'dist'/'release.json').read_text())['sections']
    for href in page.locator('a[href^="#"]').evaluate_all('(links) => links.map(a => a.getAttribute("href"))'):
        assert page.locator(href).count() == 1, href
    assert page.evaluate('document.documentElement.scrollWidth <= window.innerWidth')
    expect(page.get_by_role('link', name='京公网安备11010602203425号', exact=True)).to_have_attribute('href', 'https://beian.mps.gov.cn/#/query/webSearch?code=11010602203425')
    assert page.locator('img').evaluate_all('(items) => items.every(i => i.complete && i.naturalWidth > 0)')
    expect(page.locator('#compatibility')).to_contain_text('会被忽略')
    expect(page.locator('#task-management')).to_contain_text('cancel_requested')
    expect(page.locator('#asset-api')).to_contain_text('Idempotency-Key')
    expect(page.locator('#uploads')).to_contain_text('not_ingested')
    expect(page.locator('#capabilities')).to_contain_text('seedance-alternate')
    page.locator('#intro-search').click()
    expect(page.locator('#search-dialog')).to_be_visible()
    page.locator('#search-input').fill('metadata')
    assert page.locator('#search-results a').count() > 0
    page.keyboard.press('Escape')
    page.screenshot(path=str(out/'desktop.png'), full_page=False)
    page.locator('#tab-python').click()
    expect(page.locator('#panel-python')).to_be_visible()
    expect(page.locator('#panel-curl')).to_be_hidden()
    page.locator('#tab-python').press('ArrowRight')
    expect(page.locator('#panel-js')).to_be_visible()
    page.locator('#panel-js .copy').click()
    copied = page.evaluate('navigator.clipboard.readText()')
    assert 'process.env.JDOTOKEN_API_KEY' in copied
    page.locator('#search-open').click()
    page.locator('#search-input').fill('图片参考')
    assert page.locator('#search-results a').count() > 0
    page.locator('#search-results a').filter(has_text='使用图片参考').click()
    expect(page.locator('#search-dialog')).not_to_be_visible()
    assert page.url.endswith('#references')
    page.keyboard.press('Control+k')
    page.locator('#search-input').fill('this-query-has-no-matches-987654')
    expect(page.locator('#search-count')).to_contain_text('没有找到')
    page.keyboard.press('Escape')
    page.set_viewport_size({'width':390,'height':844})
    page.goto(args.url+'#overview', wait_until='networkidle')
    assert page.evaluate('document.documentElement.scrollWidth <= window.innerWidth')
    page.locator('#menu-button').click()
    expect(page.locator('#sidebar')).to_be_visible()
    page.locator('#sidebar a[href="#polling"]').click()
    expect(page.locator('#sidebar')).to_be_hidden()
    assert page.url.endswith('#polling')
    page.screenshot(path=str(out/'mobile.png'), full_page=False)
    page.set_viewport_size({'width':320,'height':700})
    assert page.evaluate('document.documentElement.scrollWidth <= window.innerWidth')
    assert not errors, errors
    assert not failures, failures
    assert all(r['method']=='GET' for r in requests)
    assert not any('/v1/' in r['url'] or '/api/' in r['url'] for r in requests)
    result={'url':args.url,'sections':section_count,'examples':len(examples),'checks':['example syntax and curl payloads','all anchors','desktop layout','320/390px layout','keyboard tabs','clipboard','search results and empty state','mobile menu','no API calls or JS errors'],'passed':True}
    (out/'result.json').write_text(json.dumps(result,ensure_ascii=False,indent=2))
    print(json.dumps(result,ensure_ascii=False,indent=2))
    browser.close()
