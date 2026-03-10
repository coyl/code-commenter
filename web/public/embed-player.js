(() => {
  function readConfig(scriptEl) {
    const srcUrl = new URL(scriptEl.src, window.location.href);
    const dataset = scriptEl.dataset || {};
    const jobId =
      dataset.jobId ||
      srcUrl.searchParams.get("jobId") ||
      srcUrl.searchParams.get("job_id") ||
      "";

    const width = dataset.width || "100%";
    const height = dataset.height || "640";
    const minHeight = dataset.minHeight || "360";
    const target = dataset.target || "";
    const autoplay = (dataset.autoplay || "0").toLowerCase();

    return {
      origin: srcUrl.origin,
      jobId: jobId.trim(),
      width,
      height,
      minHeight,
      target,
      autoplay: autoplay === "1" || autoplay === "true" || autoplay === "yes",
    };
  }

  function resolveMount(scriptEl, targetSelector) {
    if (targetSelector) {
      const targetEl = document.querySelector(targetSelector);
      if (targetEl) return targetEl;
    }

    const mount = document.createElement("div");
    mount.className = "code-commenter-embed";
    scriptEl.parentNode?.insertBefore(mount, scriptEl.nextSibling);
    return mount;
  }

  function renderError(mountEl, message) {
    mountEl.textContent = message;
    mountEl.style.fontFamily = "system-ui, -apple-system, sans-serif";
    mountEl.style.fontSize = "14px";
    mountEl.style.lineHeight = "1.4";
    mountEl.style.color = "#b91c1c";
    mountEl.style.background = "#fef2f2";
    mountEl.style.border = "1px solid #fecaca";
    mountEl.style.borderRadius = "8px";
    mountEl.style.padding = "10px 12px";
  }

  function mountEmbed(scriptEl) {
    const cfg = readConfig(scriptEl);
    const mountEl = resolveMount(scriptEl, cfg.target);
    if (!mountEl) return;

    if (!cfg.jobId) {
      renderError(
        mountEl,
        "Code Commenter embed error: missing job id. Set data-job-id on the script tag."
      );
      return;
    }

    const iframeUrl = new URL(`/embed/${encodeURIComponent(cfg.jobId)}`, cfg.origin);
    if (cfg.autoplay) iframeUrl.searchParams.set("autoplay", "1");

    const iframe = document.createElement("iframe");
    iframe.src = iframeUrl.toString();
    iframe.loading = "lazy";
    iframe.allow = "autoplay";
    iframe.title = "Code Commenter Player";
    iframe.style.width = cfg.width;
    iframe.style.height = /^\d+$/.test(cfg.height) ? `${cfg.height}px` : cfg.height;
    iframe.style.minHeight = /^\d+$/.test(cfg.minHeight) ? `${cfg.minHeight}px` : cfg.minHeight;
    iframe.style.border = "0";
    iframe.style.display = "block";
    iframe.style.borderRadius = "10px";
    iframe.referrerPolicy = "strict-origin-when-cross-origin";

    mountEl.innerHTML = "";
    mountEl.appendChild(iframe);
  }

  function init() {
    const scripts = document.querySelectorAll("script[data-code-commenter-embed]");
    if (scripts.length > 0) {
      scripts.forEach((s) => mountEmbed(s));
      return;
    }

    if (document.currentScript) {
      mountEmbed(document.currentScript);
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
