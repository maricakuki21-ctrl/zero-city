import { onBeforeUnmount, onMounted } from 'vue'

export function useLightPageShell(): void {
  let restoreThemeShell: (() => void) | null = null

  function applyLightPageShell(): void {
    if (typeof document === 'undefined') return

    const root = document.documentElement
    const body = document.body
    const previousTheme = root.dataset.theme
    const previousClasses = {
      dark: root.classList.contains('dark'),
      noir: root.classList.contains('theme-noir'),
      daylight: root.classList.contains('theme-daylight'),
      lightRoot: root.classList.contains('light-page-root'),
      lightBody: body.classList.contains('light-page-shell')
    }

    root.dataset.theme = 'daylight'
    root.classList.remove('dark', 'theme-noir')
    root.classList.add('theme-daylight', 'light-page-root')
    body.classList.add('light-page-shell')

    restoreThemeShell = () => {
      if (previousTheme === undefined) {
        delete root.dataset.theme
      } else {
        root.dataset.theme = previousTheme
      }

      root.classList.toggle('dark', previousClasses.dark)
      root.classList.toggle('theme-noir', previousClasses.noir)
      root.classList.toggle('theme-daylight', previousClasses.daylight)
      root.classList.toggle('light-page-root', previousClasses.lightRoot)
      body.classList.toggle('light-page-shell', previousClasses.lightBody)
    }
  }

  onMounted(applyLightPageShell)
  onBeforeUnmount(() => {
    restoreThemeShell?.()
    restoreThemeShell = null
  })
}
