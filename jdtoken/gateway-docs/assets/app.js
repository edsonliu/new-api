'use strict';
const menu = document.querySelector('#menu-button');
const sidebar = document.querySelector('#sidebar');
function closeMenu() {
  sidebar.classList.remove('open');
  menu.setAttribute('aria-expanded', 'false');
}
menu.addEventListener('click', () => {
  const open = !sidebar.classList.contains('open');
  sidebar.classList.toggle('open', open);
  menu.setAttribute('aria-expanded', String(open));
});
sidebar.querySelectorAll('a').forEach(link => link.addEventListener('click', closeMenu));
document.addEventListener('keydown', event => {
  if (event.key === 'Escape' && sidebar.classList.contains('open')) {
    closeMenu(); menu.focus();
  }
});
const toast = document.querySelector('#toast');
let toastTimeout;
document.querySelectorAll('.copy').forEach(button => {
  button.addEventListener('click', async () => {
    const source = button.closest('.code-block').querySelector('code').textContent;
    try {
      await navigator.clipboard.writeText(source);
      toast.textContent = '代码已复制';
    } catch {
      toast.textContent = '无法访问剪贴板，请选中代码手动复制';
    }
    clearTimeout(toastTimeout);
    toastTimeout = setTimeout(() => { toast.textContent = ''; }, 2500);
  });
});
document.querySelectorAll('[role="tablist"]').forEach(list => {
  const tabs = [...list.querySelectorAll('[role="tab"]')];
  function activate(tab) {
    tabs.forEach(item => {
      const selected = item === tab;
      item.setAttribute('aria-selected', String(selected));
      item.tabIndex = selected ? 0 : -1;
      document.getElementById(item.getAttribute('aria-controls')).hidden = !selected;
    });
  }
  tabs.forEach((tab, index) => {
    tab.addEventListener('click', () => activate(tab));
    tab.addEventListener('keydown', event => {
      let next;
      if (event.key === 'ArrowRight') next = tabs[(index + 1) % tabs.length];
      if (event.key === 'ArrowLeft') next = tabs[(index + tabs.length - 1) % tabs.length];
      if (event.key === 'Home') next = tabs[0];
      if (event.key === 'End') next = tabs[tabs.length - 1];
      if (next) { event.preventDefault(); activate(next); next.focus(); }
    });
  });
});
const sections = [...document.querySelectorAll('main section')];
const sideLinks = [...sidebar.querySelectorAll('a')];
let scrollQueued = false;
function updateActive() {
  const current = [...sections].reverse().find(section => section.getBoundingClientRect().top < 180) || sections[0];
  sideLinks.forEach(link => {
    if (link.hash === '#' + current.id) link.setAttribute('aria-current', 'true');
    else link.removeAttribute('aria-current');
  });
  scrollQueued = false;
}
window.addEventListener('scroll', () => {
  if (!scrollQueued) { scrollQueued = true; requestAnimationFrame(updateActive); }
}, { passive: true });
updateActive();
const dialog = document.querySelector('#search-dialog');
dialog.addEventListener('keydown', event => {
  if (event.key === 'Escape') { event.preventDefault(); dialog.close(); }
});
const input = document.querySelector('#search-input');
const results = document.querySelector('#search-results');
const count = document.querySelector('#search-count');
// textContent includes inactive code tabs, so all examples remain searchable.
const searchIndex = sections.map(section => ({
  id: section.id, title: section.dataset.title,
  text: section.textContent.replace(/\s+/g, ' ').trim(),
}));
function search() {
  const term = input.value.trim().toLowerCase();
  const matches = searchIndex.filter(item => !term || (item.title + item.text).toLowerCase().includes(term));
  results.replaceChildren();
  count.textContent = matches.length ? `${matches.length} 个相关章节` : '没有找到相关章节，请尝试“视频”“密钥”或“模型”。';
  matches.forEach(item => {
    const li = document.createElement('li');
    const link = document.createElement('a');
    link.href = '#' + item.id;
    const title = document.createElement('strong'); title.textContent = item.title;
    const snippet = document.createElement('span');
    const start = Math.max(0, item.text.toLowerCase().indexOf(term) - 25);
    snippet.textContent = (start ? '…' : '') + item.text.slice(start, start + 110) + '…';
    link.append(title, snippet); li.append(link); results.append(li);
    link.addEventListener('click', () => { dialog.close(); closeMenu(); });
  });
}
function openSearch() {
  closeMenu();
  if (!dialog.open) dialog.showModal();
  search(); input.focus();
}
document.querySelector('#search-open').addEventListener('click', openSearch);
document.querySelector('#intro-search').addEventListener('click', openSearch);
document.querySelector('#search-close').addEventListener('click', () => dialog.close());
input.addEventListener('input', search);
input.addEventListener('keydown', event => {
  if (event.key === 'ArrowDown') { event.preventDefault(); results.querySelector('a')?.focus(); }
});
document.addEventListener('keydown', event => {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault(); openSearch();
  }
});
