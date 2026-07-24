/** Full-viewport centered menu shell (Login / Lobby). */

export function mountMenuOverlay(html: string): HTMLElement {
  const overlay = document.createElement('div');
  overlay.className = 'menu-screen-overlay';
  overlay.innerHTML = html;
  document.body.appendChild(overlay);
  return overlay;
}

export function unmountMenuOverlay(overlay: HTMLElement | null | undefined): void {
  overlay?.remove();
}

/** Non-blocking toast (never uses window.alert). */
export function showDomToast(
  message: string,
  opts?: { title?: string; tone?: 'info' | 'error'; durationMs?: number }
): void {
  const tone = opts?.tone ?? 'info';
  const duration = opts?.durationMs ?? 2800;
  const title = opts?.title;

  let host = document.getElementById('dom-toast-host');
  if (!host) {
    host = document.createElement('div');
    host.id = 'dom-toast-host';
    host.className = 'dom-toast-host';
    document.body.appendChild(host);
  }

  const el = document.createElement('div');
  el.className = `dom-toast dom-toast--${tone}`;
  el.setAttribute('role', 'status');
  el.innerHTML = title
    ? `<div class="dom-toast-title">${escapeHtml(title)}</div><div>${escapeHtml(message)}</div>`
    : `<div>${escapeHtml(message)}</div>`;
  host.appendChild(el);

  requestAnimationFrame(() => el.classList.add('dom-toast--show'));

  window.setTimeout(() => {
    el.classList.remove('dom-toast--show');
    window.setTimeout(() => el.remove(), 220);
  }, duration);
}

/** Set / clear an inline error banner inside a menu overlay. */
export function setOverlayError(overlay: HTMLElement | null, message: string): void {
  if (!overlay) return;
  let box = overlay.querySelector('.error-message') as HTMLElement | null;
  if (!message) {
    box?.remove();
    return;
  }
  if (!box) {
    box = document.createElement('div');
    box.className = 'error-message';
    const container = overlay.querySelector('.glass-container');
    const h1 = container?.querySelector('h1, .lobby-room-info, .login-title-row');
    if (container && h1?.nextSibling) {
      container.insertBefore(box, h1.nextSibling);
    } else if (container) {
      container.prepend(box);
    } else {
      overlay.prepend(box);
    }
  }
  box.textContent = message;
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}
