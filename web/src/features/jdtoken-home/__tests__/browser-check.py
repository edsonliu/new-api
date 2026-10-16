"""Browser regression for the JDOTOKEN homepage, using explicit API fixtures only.

Requires Python playwright and Chrome (or playwright's installed Chromium).
No production connection or paid model invocation is made.
"""
import argparse
import json
import re
import shutil
from pathlib import Path
from urllib.parse import urlsplit

from playwright.sync_api import expect, sync_playwright

parser = argparse.ArgumentParser()
parser.add_argument('--url', default='http://127.0.0.1:4173')
parser.add_argument('--output', default='/tmp/jdtoken-preview')
parser.add_argument('--upstream', action='store_true')
args = parser.parse_args()
output = Path(args.output)
output.mkdir(parents=True, exist_ok=True)

with sync_playwright() as p:
    browser = p.chromium.launch(executable_path=shutil.which('google-chrome') or None,
                                headless=True, args=['--no-sandbox'])
    context = browser.new_context(viewport={'width': 1440, 'height': 1000},
                                  locale='zh-CN', reduced_motion='reduce',
                                  permissions=['clipboard-read', 'clipboard-write'])
    errors = []
    external_requests = []
    state = {'long_model': False}
    names = ['DeepSeek-V3.2', 'DeepSeek-V4-Flash', 'Qwen/Qwen3.6-35B-A3B', 'GLM-5',
             'MiniMax-M3', 'Kimi-K2.7-Code', 'Qwen3.8-27B', 'DeepSeek-V4-Pro']

    def serve_api(route):
        path = route.request.url.split('/api/', 1)[-1].split('?', 1)[0]
        response = {'success': True, 'data': ''}
        if path == 'status':
            response['data'] = {
                'system_name': 'JDOTOKEN', 'logo': '/jdtoken/mark-square-20261012.png', 'register_enabled': True,
                'docs_link': 'https://docs.newapi.pro',
                'HeaderNavModules': json.dumps({'pricing': {'enabled': True, 'requireAuth': False},
                                               'rankings': False, 'about': False}),
                'server_address': 'https://gateway.example.com',
            }
        elif path == 'setup':
            response['data'] = {'status': True}
        elif 'refresh' in path or path == 'user/self':
            route.fulfill(status=401, content_type='application/json',
                          body=json.dumps({'success': False, 'message': 'Not signed in'}))
            return
        elif path == 'pricing':
            model_names = names
            if state['long_model']:
                model_names = ['provider/' + 'long-model-name-' * 16]
            response = {
                'success': True,
                'data': [{'id': i + 1, 'model_name': name, 'description': '', 'quota_type': 0,
                          'model_ratio': 1, 'completion_ratio': 1, 'enable_groups': ['default'],
                          'supported_endpoint_types': ['openai']}
                         for i, name in enumerate(model_names)],
                'vendors': [], 'group_ratio': {}, 'usable_group': {},
                'supported_endpoint': {}, 'auto_groups': [],
            }
        route.fulfill(status=200, content_type='application/json', body=json.dumps(response))

    context.route('**/api/**', serve_api)
    page = context.new_page()
    page.on('pageerror', lambda error: errors.append(str(error)))
    page.on('request', lambda request: external_requests.append(request.url)
            if urlsplit(request.url).netloc not in ['', urlsplit(args.url).netloc]
            and request.url.startswith(('http:', 'https:')) else None)
    page.goto(args.url, wait_until='networkidle')
    page.locator('h1').wait_for()
    if args.upstream:
        assert '模型调用，从未如此简单' not in page.locator('h1').inner_text()
        assert not page.locator('img[src="/jdtoken/hero.webp"]').count()
        assert not errors, errors
        print('PASS: production upstream build switch')
        browser.close()
        raise SystemExit(0)

    expect(page.get_by_role('heading', name='模型调用，从未如此简单')).to_be_visible()
    expect(page.get_by_role('heading', name=names[0], exact=True)).to_be_visible()
    assert not page.locator('a[href*="jdotoken.com"]').count()
    assert not page.locator('a[href^="/portal/"]').count()
    expect(page.get_by_role('link', name='查看 Qwen/Qwen3.6-35B-A3B 详情')).to_have_attribute(
        'href', '/pricing/Qwen%2FQwen3.6-35B-A3B')
    expect(page.get_by_role('link', name='查看订阅套餐与账户余额')).to_have_attribute('href', '/wallet')
    assert not page.locator('a[href="https://github.com/QuantumNous/new-api"]').count()
    assert 'new api' not in page.locator('body').inner_text().lower()
    footer = page.get_by_role('contentinfo')
    expect(footer).to_contain_text('及第嘉序(北京)智算科技有限公司')
    filing = footer.get_by_role('link', name='京ICP备2026065372号-1', exact=True)
    expect(filing).to_have_attribute('href', 'https://beian.miit.gov.cn/')
    expect(filing).to_have_attribute('target', '_blank')
    expect(filing).to_have_attribute('rel', 'noopener noreferrer')
    police = footer.get_by_role('link', name='京公网安备11010602203425号', exact=True)
    expect(police).to_have_attribute('href', 'https://beian.mps.gov.cn/#/query/webSearch?code=11010602203425')
    expect(police).to_have_attribute('rel', 'noopener noreferrer')
    expect(police.locator('img')).to_have_attribute('src', '/jdtoken/police-beian.png')
    expect(footer.locator('img[src="/jdtoken/logo-horizontal.png"]')).to_be_visible()
    page.get_by_role('button', name='复制代码', exact=True).click()
    copied = page.evaluate('navigator.clipboard.readText()')
    assert 'https://gateway.example.com/v1' in copied
    assert names[0] in copied and 'os.environ["OPENAI_API_KEY"]' in copied
    assert '/openai/v1' not in copied
    page.evaluate('window.scrollTo(0, 0)')
    page.evaluate('document.fonts.ready')
    page.screenshot(path=str(output / 'desktop.png'), full_page=True)

    for width in [320, 390, 768, 1440]:
        page.set_viewport_size({'width': width, 'height': 900})
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), f'overflow at {width}'
        columns = page.locator('#models article').evaluate_all(
            '(cards) => [...new Set(cards.map(c => Math.round(c.getBoundingClientRect().x)))].length')
        assert columns == (1 if width < 640 else 2 if width < 1024 else 4), (width, columns)

    page.set_viewport_size({'width': 390, 'height': 844})
    page.evaluate('window.scrollTo(0, 0)')
    menu = page.get_by_role('button', name=re.compile('导航菜单|navigation menu'))
    menu.focus()
    page.keyboard.press('Enter')
    # The upstream header represents open state through its overlay styles.
    overlay = page.locator('header + div').filter(has=page.locator('nav'))
    expect(overlay).to_have_css('opacity', '1')
    expect(overlay).to_have_css('pointer-events', 'auto')
    menu.focus()
    page.keyboard.press('Enter')
    expect(overlay).to_have_css('opacity', '0')
    expect(overlay).to_have_css('pointer-events', 'none')
    page.screenshot(path=str(output / 'mobile.png'), full_page=True)
    assert page.locator('img').evaluate_all('(images) => images.every(i => i.complete && i.naturalWidth > 0)')
    transition = page.locator('#models article').first.evaluate('(el) => getComputedStyle(el).transitionDuration')
    assert transition == '0s', transition

    page.set_viewport_size({'width': 1440, 'height': 1000})
    page.evaluate('document.documentElement.classList.add("dark")')
    expect(page.get_by_role('heading', name='模型调用，从未如此简单')).to_be_visible()
    expect(footer.locator('img[src="/jdtoken/logo-horizontal-white.png"]')).to_be_visible()
    page.screenshot(path=str(output / 'dark.png'), full_page=True)

    page.goto(args.url + '/portal/?source=legacy#models', wait_until='networkidle')
    expect(page).to_have_url(re.compile(r'/\?source=legacy#models$'))
    expect(page.get_by_role('heading', name='模型调用，从未如此简单')).to_be_visible()

    state['long_model'] = True
    page.set_viewport_size({'width': 390, 'height': 844})
    page.reload(wait_until='networkidle')
    expect(page.get_by_role('heading', name='provider/' + 'long-model-name-' * 16,
                           exact=True)).to_be_visible()
    assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), 'long model overflows'
    assert not errors, errors
    assert not external_requests, external_requests
    for route in ['/', '/sign-in', '/sign-up', '/pricing']:
        page.goto(args.url + route, wait_until='networkidle')
        expect(page).to_have_title('JDOTOKEN')
        expect(page.locator('body')).to_contain_text('JDOTOKEN')
        assert not re.search(r'jdtoken', page.locator('body').inner_text(), re.IGNORECASE)
        for width in [320, 390, 1440]:
            page.set_viewport_size({'width': width, 'height': 900})
            assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), (route, width)
            logos = page.locator('img[src*="/jdtoken/mark"], img[src*="/jdtoken/logo"], img[src*="/jdtoken/police"]').evaluate_all(
                '(images) => images.filter(i => i.getBoundingClientRect().width > 0).map(i => ({width:i.width, height:i.height, naturalWidth:i.naturalWidth, naturalHeight:i.naturalHeight}))')
            assert logos, route
            for logo in logos:
                assert logo['naturalWidth'] > 0, (route, logo)
                assert abs(logo['width'] / logo['height'] - logo['naturalWidth'] / logo['naturalHeight']) < 0.1, (route, logo)
        page.screenshot(path=str(output / ('brand-' + (route.strip('/') or 'home') + '.png')))
    print('PASS: catalog links, clipboard, assets, 4 responsive widths, keyboard menu, reduced motion, dark theme, legacy redirect, long model ID; no browser errors or external requests')
    print('Preview screenshots:', output)
    browser.close()
