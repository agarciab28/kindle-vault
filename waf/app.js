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

  // Account actions only. The body is form-encoded: the password never goes in the URL.
  function post(path, fields, ok, fail) {
    var xhr = new XMLHttpRequest();
    var body = [];
    for (var k in fields) {
      if (fields.hasOwnProperty(k)) { body.push(encodeURIComponent(k) + '=' + encodeURIComponent(fields[k])); }
    }
    xhr.open('POST', API + path + '?k=' + encodeURIComponent(window.VAULT_TOKEN || ''), true);
    xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
    xhr.timeout = 40000;
    xhr.onreadystatechange = function () {
      if (xhr.readyState !== 4) { return; }
      var r = null;
      if (xhr.status === 200) { try { r = JSON.parse(xhr.responseText); } catch (e) { r = null; } }
      if (r) { ok(r); } else { fail(); }
    };
    xhr.send(body.join('&'));
  }

  function accountMsg(html) {
    var el = document.getElementById('loginmsg');
    if (el) { el.innerHTML = html; }
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
    else if (h === '/account') {
      show({ title: 'Cuenta', html: '<p>Revisando la sesión de Obsidian…</p>' });
      get('/api/account', show);
    }
    else if (h === '/account/login') { get('/api/account?form=1', show); }
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
    html += '</div><p><a class="btn" href="#/">Ir al inicio</a>';
    if (!state.running && !state.lastOk) { html += ' <a class="btn" href="#/account">Cuenta</a>'; }
    html += '</p>';
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
    login: function (form) {
      var btn = form.getElementsByTagName('button')[0];
      if (btn.disabled) { return false; }
      btn.disabled = true;
      accountMsg('<p><strong>Iniciando sesión…</strong></p>');
      post('/api/login', { email: form.email.value, password: form.password.value, mfa: form.mfa.value, accept: form.accept.value }, function (r) {
        btn.disabled = false;
        form.mfa.value = '';
        if (r.ok) {
          form.password.value = '';
          accountMsg('<p><strong>' + escapeHtml(r.message) + '</strong> Sincronizando…</p>');
          setTimeout(function () { location.hash = '#/sync'; }, 1500);
          return;
        }
        accountMsg('<p class="alert"><strong>' + escapeHtml(r.message) + '</strong></p>');
        if (r.need2fa) { form.mfa.focus(); }
      }, function () {
        btn.disabled = false;
        accountMsg('<p class="alert"><strong>No hubo respuesta del servicio.</strong> Vuelve a intentarlo.</p>');
      });
      return false;
    },
    accept: function () { // the third-party notice in the login form
      var btn = document.getElementById('lf-accept');
      var input = btn && btn.parentNode.parentNode.getElementsByTagName('input')[0];
      if (!input) { return; }
      var on = input.value !== '1';
      input.value = on ? '1' : '';
      btn.className = on ? 'btn on' : 'btn';
      btn.innerHTML = on ? 'Aceptado' : 'Toca aquí para aceptar';
    },
    logout: function (confirmed) {
      if (!confirmed) {
        accountMsg('<p>Este Kindle dejará de actualizar la bóveda hasta que vuelvas a iniciar sesión. Las notas descargadas se quedan.</p>' +
          '<p><a class="btn" href="javascript:app.logout(true)">Sí, cerrar sesión</a></p>');
        return;
      }
      accountMsg('<p>Cerrando sesión…</p>');
      post('/api/logout', {}, function (r) {
        if (r.ok) { show({ title: 'Cuenta', html: '<div class="syncbox"><p><strong>' + escapeHtml(r.message) + '</strong></p></div><p><a class="btn" href="#/account/login">Iniciar sesión</a> <a class="btn" href="#/">Ir al inicio</a></p>' }); }
        else { accountMsg('<p class="alert"><strong>' + escapeHtml(r.message) + '</strong></p>'); }
      }, function () {
        accountMsg('<p class="alert"><strong>No hubo respuesta del servicio.</strong></p>');
      });
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
