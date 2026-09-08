export async function copyText(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    try {
      await Promise.race([
        navigator.clipboard.writeText(text),
        new Promise((_, reject) => window.setTimeout(() => reject(new Error('clipboard timeout')), 1000)),
      ])
      return
    } catch {
      // Fall back for non-HTTPS origins or denied clipboard permissions.
    }
  }

  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'true')
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  textarea.style.top = '0'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  textarea.setSelectionRange(0, textarea.value.length)

  const copied = document.execCommand('copy')
  document.body.removeChild(textarea)

  if (!copied) {
    window.prompt('复制失败，请手动复制以下内容：', text)
    throw new Error('copy failed')
  }
}
