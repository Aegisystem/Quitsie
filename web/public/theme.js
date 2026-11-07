// Applies the theme chosen on this device before the first paint, so a forced
// theme never flashes the system one. It is a file, not an inline script,
// because the CSP allows only 'self'. Mirrors src/ui/theme.ts (key and colours).
;(function () {
  var choice = null
  try {
    choice = localStorage.getItem('quitsie.theme')
  } catch (e) {
    return
  }
  if (choice !== 'light' && choice !== 'dark') return
  document.documentElement.dataset.theme = choice
  var color = choice === 'dark' ? '#0a0b0d' : '#1f2328'
  var metas = document.querySelectorAll('meta[name="theme-color"]')
  for (var i = 0; i < metas.length; i++) metas[i].content = color
})()
