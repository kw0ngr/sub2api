export function showBootstrapFailure(): void {
  const message = document.querySelector('[data-bootstrap-message]')
  if (message) {
    message.textContent = '页面加载失败，请刷新重试。'
  }
}
