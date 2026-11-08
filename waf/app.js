/* Bóveda: Kindle WAF shell. ES5 only (the Kindle web engine is old). */
(function () {
  var API = 'http://127.0.0.1:8317';
  var APP_ID = 'com.alexgar.vault';
  var contentEl = document.getElementById('content');
  var pollTimer = null;

  function get(path, ok, fail) {
    var xhr = new XMLHttpRequest();
    // vaultd rejects requests without the per-start token published in token.js.
    var sep = path.indexOf('?') >= 0 ? '&' : '?';
    xhr.open('GET', API + path + sep + 'k=' + encodeURIComponent(window.VAULT_TOKEN || ''), true);
    xhr.timeout = 15000;
    xhr.onreadystatechange = function () {
      if (xhr.readyState !== 4) { return; }
      if (xhr.status === 200) {
        try { ok(JSON.parse(xhr.responseText)); } catch (e) { (fail || offline)(); }
      } else {
        (fail || offline)();
      }
    };
    xhr.send(null);
  }

  function offline() {
    show({
      title: 'Bóveda',
      html: '<p><strong>El servicio de la bóveda no está corriendo.</strong></p>' +
        '<p>Abre la app desde el libro <em>Bóveda</em> de tu biblioteca; ese lanzador lo inicia.</p>' +
        '<p><a class="btn" href="javascript:app.retry()">Reintentar</a></p>'
    });
  }

  function show(page) {
    document.title = page.title || 'Bóveda';
    contentEl.innerHTML = page.html;
    window.scrollTo(0, 0);
    setupChrome(page.title || 'Bóveda');
  }

  function stopPolling() {
    if (pollTimer) { clearTimeout(pollTimer); pollTimer = null; }
  }

  function route() {
    stopPolling();
    var h = location.hash.replace(/^#/, '') || '/';
    var m;
    if (h === '/' ) { get('/api/home', show); }
    else if ((m = h.match(/^\/note\/(.+)$/))) { get('/api/note?p=' + encodeURIComponent(decodeURIComponent(m[1])), show); }
    else if ((m = h.match(/^\/folder\/(.+)$/))) { get('/api/folder?p=' + encodeURIComponent(decodeURIComponent(m[1])), show); }
    else if ((m = h.match(/^\/tag\/(.+)$/))) { get('/api/tag?t=' + encodeURIComponent(decodeURIComponent(m[1])), show); }
    else if (h === '/tags') { get('/api/tags', show); }
    else if ((m = h.match(/^\/search(?:\/(.*))?$/))) {
      get('/api/search?q=' + encodeURIComponent(m[1] ? decodeURIComponent(m[1]) : ''), function (p) {
        show(p);
        var input = contentEl.getElementsByTagName('input')[0];
        if (input && !m[1]) { input.focus(); }
      });
    }
    else if (h === '/sync') { startSync(); }
    else { location.hash = '#/'; }
  }

  function syncView(state, extra) {
    var html = '<div class="syncbox">';
    if (state.running) {
      html += '<p><strong>Sincronizando con Obsidian Sync…</strong></p><p class="muted">Solo descarga; nunca sube cambios.</p>';
    } else if (state.lastOk) {
      html += '<p><strong>Listo.</strong> ' + state.lastChanged + ' archivos actualizados.</p>';
    } else {
      html += '<p><strong>No se pudo sincronizar.</strong></p><pre class="err">' + escapeHtml(state.lastError || extra || '') + '</pre>';
    }
    html += '</div><p><a class="btn" href="#/">Ir al inicio</a></p>';
    show({ title: 'Sincronizar', html: html });
  }

  function startSync() {
    show({ title: 'Sincronizar', html: '<p>Conectando…</p>' });
    get('/api/sync', function (r) {
      if (r.error) {
        syncView({ running: false, lastOk: false, lastError: r.error });
        return;
      }
      poll();
    });
  }

  function poll() {
    get('/api/status', function (r) {
      syncView(r.state);
      if (r.state.running) { pollTimer = setTimeout(poll, 2500); }
    });
  }

  function escapeHtml(s) {
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  /* Diagnostics: lines end up in /mnt/us/vault-app/vaultd.log */
  function dlog(msg) {
    get('/api/log?m=' + encodeURIComponent(msg), function () {}, function () {});
  }

  /* System title bar. The back arrow is configured like Amazon's com.lab126.mfa
     (searchBar profile, id "back", handling "notifyapp"), which works for apps
     outside the Decanter whitelist; topNavBar sets the title on new firmware. */
  function setupChrome(title) {
    try {
      if (window.kindle && kindle.messaging) {
        kindle.messaging.sendMessage('com.lab126.chromebar', 'configureChrome', {
          appId: APP_ID,
          searchBar: { clientParams: { profile: { name: 'default', buttons: [
            { id: 'back', state: 'enabled', handling: 'notifyapp' }
          ] } } },
          topNavBar: { template: 'title', title: title, buttons: [
            { id: 'KPP_BACK', state: 'enabled', handling: 'notifyApp' },
            { id: 'KPP_CLOSE', state: 'enabled', handling: 'system' }
          ] }
        });
      }
    } catch (e) { dlog('configureChrome failed: ' + e); }
  }

  function onBarButton(property, id) {
    var s = typeof id === 'object' ? JSON.stringify(id) : String(id);
    dlog('bar event ' + property + ' ' + s);
    if (/back/i.test(s)) { app.back(); }
  }

  function initKindle() {
    try {
      if (window.kindle && kindle.messaging) {
        // New firmware (KPPChrome) sets the LIPC property "invokeAction" = KPP_BACK on
        // the app; receiveMessage publishes that property. Older chrome uses
        // "searchBarButtonSelected" = back.
        var ok1 = kindle.messaging.receiveMessage('invokeAction', onBarButton);
        var ok2 = kindle.messaging.receiveMessage('searchBarButtonSelected', onBarButton);
        dlog('subscribed back button: invokeAction=' + ok1 + ' searchBar=' + ok2);
      }
      setupChrome('Bóveda');
      if (window.kindle && kindle.gestures) {
        kindle.gestures.onswipe = function (direction) {
          if (direction === 'up') { app.page(1); }
          else if (direction === 'down') { app.page(-1); }
        };
      }
    } catch (e) { /* not on a Kindle */ }
  }

  window.app = {
    back: function () {
      // Own history stack: the Kindle web view's history.back() is unreliable.
      stack.pop(); // current page
      var prev = stack.pop() || '#/';
      if (location.hash === prev) { stack.push(prev); route(); } else { location.hash = prev; }
    },
    exit: function () { get('/api/exit', function () {}, function () {}); },
    retry: route,
    page: function (dir) {
      var h = window.innerHeight || document.documentElement.clientHeight;
      window.scrollBy(0, dir * Math.round(h * 0.85));
    },
    search: function (form) {
      var q = form.q.value.replace(/^\s+|\s+$/g, '');
      location.hash = '#/search/' + encodeURIComponent(q);
      return false;
    }
  };

  var stack = [];
  function navigate() {
    var h = location.hash || '#/';
    if (h !== '#/sync' && stack[stack.length - 1] !== h) { stack.push(h); }
    if (stack.length > 50) { stack.shift(); }
    route();
  }
  window.onhashchange = navigate;
  initKindle();
  navigate();
})();
