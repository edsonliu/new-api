"""Verify application navigation, availability and responsive cards with local API fixtures."""
import argparse
import json
import re
import shutil
from pathlib import Path
from urllib.parse import urlsplit

from playwright.sync_api import expect, sync_playwright

parser = argparse.ArgumentParser()
parser.add_argument('--url', default='http://127.0.0.1:4190')
parser.add_argument('--output', default='/tmp/jdtoken-apps-preview')
args = parser.parse_args()
output = Path(args.output)
output.mkdir(parents=True, exist_ok=True)

with sync_playwright() as p:
    browser = p.chromium.launch(executable_path=shutil.which('google-chrome') or None,
                                headless=True, args=['--no-sandbox'])
    context = browser.new_context(viewport={'width': 1440, 'height': 1000}, locale='zh-CN', reduced_motion='reduce')
    errors, external = [], []

    def api_fixture(route):
        path = urlsplit(route.request.url).path
        data = {'success': True, 'data': ''}
        if path == '/api/status':
            data['data'] = {'system_name': 'JDOTOKEN', 'logo': '/jdtoken/mark-square-20261012.png',
                            'server_address': args.url, 'register_enabled': True,
                            'HeaderNavModules': {'pricing': True, 'rankings': True, 'docs': True, 'about': True}}
        elif path == '/api/setup':
            data['data'] = {'status': True}
        elif path == '/api/pricing':
            data = {'success': True, 'data': [], 'vendors': [], 'group_ratio': {}, 'usable_group': {},
                    'supported_endpoint': {}, 'auto_groups': []}
        elif 'refresh' in path or path == '/api/user/self':
            route.fulfill(status=401, content_type='application/json', body=json.dumps({'success': False, 'message': 'Not signed in'}))
            return
        route.fulfill(status=200, content_type='application/json', body=json.dumps(data))

    context.route('**/api/**', api_fixture)
    page = context.new_page()
    page.on('pageerror', lambda e: errors.append(str(e)))
    page.on('request', lambda r: external.append(r.url) if r.url.startswith(('http:', 'https:')) and urlsplit(r.url).netloc != urlsplit(args.url).netloc else None)
    page.goto(args.url, wait_until='networkidle')
    nav = page.locator('header nav')
    links = nav.locator('a').evaluate_all('(links) => links.map(a => a.getAttribute("href"))')
    assert links[links.index('/rankings') - 1] == '/agent-market', links
    nav.get_by_role('link', name='应用', exact=True).click()
    expect(page).to_have_url(re.compile('/agent-market/?$'))
    expect(page.get_by_role('heading', name='应用', level=1)).to_be_visible()
    expect(nav.get_by_role('link', name='应用', exact=True)).to_have_attribute('aria-current', 'page')
    cards = page.get_by_role('article')
    expect(cards).to_have_count(6)
    expect(page.get_by_role('button', name='即将上线')).to_have_count(5)
    for button in page.get_by_role('button', name='即将上线').all():
        expect(button).to_be_disabled()
    expect(page.get_by_role('link', name='视频 API 接入')).to_have_attribute('href', '/docs/#video-workflow')
    expect(page.get_by_role('link', name='素材库 API 接入')).to_have_attribute('href', '/docs/#asset-mode')
    assert not page.locator('iframe').count()
    assert not page.locator('a[href*="moyu.cn"]').count()
    checks = []
    for width in [1440, 1024, 768, 390, 320]:
        page.set_viewport_size({'width': width, 'height': 1000})
        assert page.evaluate('document.documentElement.scrollWidth <= innerWidth'), width
        columns = cards.evaluate_all('(cards) => new Set(cards.map(c => Math.round(c.getBoundingClientRect().x))).size')
        assert columns == (3 if width >= 1024 else 2 if width >= 640 else 1), (width, columns)
        assert cards.evaluate_all('(cards) => cards.every(c => c.scrollWidth <= c.clientWidth + 1)')
        checks.append({'width': width, 'columns': columns, 'no_overflow': True})
        if width in (1440, 390):
            page.screenshot(path=str(output / f'apps-{width}.png'), full_page=True)
    page.set_viewport_size({'width': 390, 'height': 844})
    page.goto(args.url, wait_until='networkidle')
    menu = page.get_by_role('button', name=re.compile('导航菜单|navigation menu'))
    menu.focus(); page.keyboard.press('Enter')
    mobile = page.locator('header + div').filter(has=page.locator('nav'))
    expect(mobile).to_have_css('pointer-events', 'auto')
    links = mobile.locator('a').evaluate_all('(links) => links.map(a => a.getAttribute("href"))')
    assert links[links.index('/rankings') - 1] == '/agent-market'
    mobile.get_by_role('link', name='应用', exact=True).click()
    expect(page.get_by_role('heading', name='应用', level=1)).to_be_visible()
    expect(mobile).to_have_css('pointer-events', 'none')
    page.reload(wait_until='networkidle')
    expect(page.get_by_role('heading', name='应用', level=1)).to_be_visible()
    launch = page.get_by_role('link', name='立即使用', exact=True)
    launch.focus(); page.keyboard.press('Enter')
    expect(page).to_have_url(re.compile('/sign-in\\?redirect=%2Fplayground'))
    page.goto(args.url + '/agent-market', wait_until='networkidle')
    page.set_viewport_size({'width': 1440, 'height': 1000})
    page.evaluate('document.documentElement.classList.add("dark")')
    page.screenshot(path=str(output / 'apps-dark.png'), full_page=True)
    assert page.locator('img').evaluate_all('(images) => images.every(i => i.complete && i.naturalWidth > 0)')
    for footer_link in ['京ICP备2026065372号-1', '京公网安备11010602203425号']:
        expect(page.get_by_role('contentinfo').get_by_role('link', name=footer_link, exact=True)).to_be_visible()
    assert not errors, errors
    assert not external, external
    result = {'passed': True, 'layouts': checks, 'menu_order_desktop_mobile': True, 'keyboard_launch_preserves_login_redirect': True,
              'direct_route_reload': True, 'no_external_site_embedding': True, 'available_and_pending_actions': True,
              'brand_and_legal_footer': True, 'no_page_errors': True}
    (output / 'result.json').write_text(json.dumps(result, indent=2))
    print(json.dumps(result, indent=2))
    browser.close()
