(function () {
  const root = document.documentElement
  const saved = localStorage.getItem('wander-site-theme') || 'system'
  const media = window.matchMedia('(prefers-color-scheme: dark)')

  function applyTheme(theme) {
    root.classList.toggle('dark', theme === 'dark' || (theme === 'system' && media.matches))
    root.dataset.theme = theme
  }

  applyTheme(saved)
  media.addEventListener('change', function () {
    if ((localStorage.getItem('wander-site-theme') || 'system') === 'system') applyTheme('system')
  })

  window.addEventListener('DOMContentLoaded', function () {
    const menuButton = document.querySelector('[data-menu-button]')
    const menu = document.querySelector('[data-menu]')
    if (menuButton && menu) {
      menuButton.addEventListener('click', function () {
        const open = menu.classList.toggle('hidden') === false
        menuButton.setAttribute('aria-expanded', String(open))
      })
    }

    document.querySelectorAll('[data-theme]').forEach(function (button) {
      button.addEventListener('click', function () {
        const next = root.dataset.theme === 'system' ? 'light' : root.dataset.theme === 'light' ? 'dark' : 'system'
        localStorage.setItem('wander-site-theme', next)
        applyTheme(next)
        button.setAttribute('aria-label', 'Color theme: ' + next + '. Activate to change.')
      })
      button.setAttribute('aria-label', 'Color theme: ' + root.dataset.theme + '. Activate to change.')
    })
  })
})()
