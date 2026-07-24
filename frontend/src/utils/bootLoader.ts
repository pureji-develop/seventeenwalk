/** Hide the HTML boot splash once Phaser is ready. */
export function hideBootLoader() {
  const el = document.getElementById('boot-loader');
  if (!el) return;
  el.classList.add('hidden');
  window.setTimeout(() => el.remove(), 400);
}
