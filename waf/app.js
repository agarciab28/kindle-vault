/* Vault: Kindle WAF shell. ES5 only (the Kindle web engine is old).
   Pages come from vaultd already in the interface language (English or Spanish;
   English by default). The few texts the shell needs on its own are in TEXT.
   The language is about the app only: notes are shown as they were written. */
(function () {
  var API = 'http://127.0.0.1:8317';
  var APP_ID = 'com.alexgar.vault';
  var contentEl = document.getElementById('content');
  var menuEl = document.getElementById('menu');
  var pollTimer = null;
  var busy = false; // a vault is being set up

  var TEXT = {
    en: {
      home: 'Home', search: 'Search', tags: 'Tags', sync: 'Sync', account: 'Account', vault: 'Vault',
      offBody: 'The vault service is not running.',
      offHint: 'Open the app from the <em>Vault</em> book in your library; that launcher starts it.',
      retry: 'Retry', checking: 'Checking your Obsidian session…', loadingVaults: 'Getting your vaults…',
      connecting: 'Connecting…', syncing: 'Syncing with Obsidian Sync…', syncHint: 'Download only; it never uploads changes.',
      done: 'Done.', files1: '1 file updated.', filesN: '{0} files updated.', syncFail: 'Could not sync.',
      goHome: 'Go to home', chooseVault: 'Choose vault',
      signingIn: 'Signing in…', syncingShort: 'Syncing…', noAnswer: 'No answer from the service.', tryAgain: 'Try again.',
      logoutConfirm: 'Signing out removes the downloaded notes from this Kindle. Nothing changes in Obsidian.',
      logoutYes: 'Yes, sign out', signingOut: 'Signing out…', signIn: 'Sign in'
    },
    es: {
      home: 'Inicio', search: 'Buscar', tags: 'Tags', sync: 'Sincronizar', account: 'Cuenta', vault: 'Bóveda',
      offBody: 'El servicio de la bóveda no está corriendo.',
      offHint: 'Abre la app desde el libro <em>Vault</em> de tu biblioteca; ese lanzador lo inicia.',
      retry: 'Reintentar', checking: 'Revisando la sesión de Obsidian…', loadingVaults: 'Consultando tus bóvedas…',
      connecting: 'Conectando…', syncing: 'Sincronizando con Obsidian Sync…', syncHint: 'Solo descarga; nunca sube cambios.',
      done: 'Listo.', files1: '1 archivo actualizado.', filesN: '{0} archivos actualizados.', syncFail: 'No se pudo sincronizar.',
      goHome: 'Ir al inicio', chooseVault: 'Elegir bóveda',
      signingIn: 'Iniciando sesión…', syncingShort: 'Sincronizando…', noAnswer: 'No hubo respuesta del servicio.', tryAgain: 'Vuelve a intentarlo.',
      logoutConfirm: 'Al cerrar sesión se quitan de este Kindle las notas descargadas. En Obsidian no cambia nada.',
      logoutYes: 'Sí, cerrar sesión', signingOut: 'Cerrando sesión…', signIn: 'Iniciar sesión'
    }
  };
  var lang = 'en';
  try {
    var saved = window.localStorage.getItem('vault.lang');
    if (TEXT[saved]) { lang = saved; }
  } catch (e) { /* no storage: English */ }

  function t(key, a) {
    var s = TEXT[lang][key];
    if (s === undefined) { s = TEXT.en[key]; }
    return String(s).replace('{0}', a === undefined ? '' : a);
  }
  function drawMenu() {
    document.documentElement.lang = lang;
    // The gear sits at the right end of the row, right under the X of the title bar.
    menuEl.innerHTML = '<a href="#/">' + t('home') + '</a> <a href="#/search">' + t('search') + '</a> ' +
      '<a href="#/tags">' + t('tags') + '</a> <a href="#/sync">' + t('sync') + '</a>' +
      '<a class="gear" href="#/settings"></a>';
  }

  function url(path) {
    // vaultd rejects requests without the per-start token published in token.js.
    return API + path + (path.indexOf('?') >= 0 ? '&' : '?') + 'k=' + encodeURIComponent(window.VAULT_TOKEN || '') + '&l=' + lang;
  }
  function get(path, ok, fail) {
    var xhr = new XMLHttpRequest();
    xhr.open('GET', url(path), true);
    xhr.timeout = 35000;
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

  // Account and vault actions only. The body is form-encoded: passwords never go in the URL.
  function post(path, fields, ok, fail, timeout) {
    var xhr = new XMLHttpRequest();
    var body = [];
    for (var k in fields) {
      if (fields.hasOwnProperty(k)) { body.push(encodeURIComponent(k) + '=' + encodeURIComponent(fields[k])); }
    }
    xhr.open('POST', url(path), true);
    xhr.setRequestHeader('Content-Type', 'application/x-www-form-urlencoded');
    xhr.timeout = timeout || 40000;
    xhr.onreadystatechange = function () {
      if (xhr.readyState !== 4) { return; }
      var r = null;
      if (xhr.status === 200) { try { r = JSON.parse(xhr.responseText); } catch (e) { r = null; } }
      if (r) { ok(r); } else { fail(); }
    };
    xhr.send(body.join('&'));
  }

  function setMsg(id, html) {
    var el = document.getElementById(id);
    if (el) { el.innerHTML = html; }
    return el;
  }
  function accountMsg(html) { setMsg('loginmsg', html); }
  function btn(href, label) { return '<a class="btn" href="' + href + '">' + label + '</a>'; }

  function offline() {
    show({
      title: 'Vault',
      html: '<p><strong>' + t('offBody') + '</strong></p><p>' + t('offHint') + '</p><p>' + btn('javascript:app.retry()', t('retry')) + '</p>'
    });
  }

  function show(page) {
    document.title = page.title || 'Vault';
    contentEl.innerHTML = page.html;
    window.scrollTo(0, 0);
    setupChrome(page.title || 'Vault');
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
      show({ title: t('account'), html: '<p>' + t('checking') + '</p>' });
      get('/api/account', show);
    }
    else if (h === '/account/login') { get('/api/account?form=1', show); }
    else if (h === '/settings') { get('/api/settings', show); }
    else if (h === '/vault') {
      show({ title: t('vault'), html: '<p>' + t('loadingVaults') + '</p>' });
      get('/api/vaults', show);
    }
    else { location.hash = '#/'; }
  }

  function syncView(state, extra, needVault) {
    var html = '<div class="syncbox">';
    if (state.running) {
      html += '<p><strong>' + t('syncing') + '</strong></p><p class="muted">' + t('syncHint') + '</p>';
    } else if (state.lastOk && !extra) {
      html += '<p><strong>' + t('done') + '</strong> ' + (state.lastChanged === 1 ? t('files1') : t('filesN', state.lastChanged)) + '</p>';
    } else {
      html += '<p><strong>' + t('syncFail') + '</strong></p><pre class="err">' + escapeHtml(extra || state.lastError || '') + '</pre>';
    }
    html += '</div><p>' + btn('#/', t('goHome'));
    if (needVault) { html += ' ' + btn('#/vault', t('chooseVault')); }
    else if (!state.running && (!state.lastOk || extra)) { html += ' ' + btn('#/account', t('account')); }
    html += '</p>';
    show({ title: t('sync'), html: html });
  }

  function startSync() {
    show({ title: t('sync'), html: '<p>' + t('connecting') + '</p>' });
    get('/api/sync', function (r) {
      if (r.error) {
        syncView({ running: false, lastOk: false }, r.error, r.needVault);
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
    return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
  }

  /* Diagnostics: lines end up in the vaultd log. */
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
      setupChrome('Vault');
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
    lang: function (code) { // the interface language; notes are not affected
      if (!TEXT[code]) { return; }
      lang = code;
      try { window.localStorage.setItem('vault.lang', code); } catch (e) { /* not remembered */ }
      drawMenu();
      route();
    },
    login: function (form) {
      var button = form.getElementsByTagName('button')[0];
      if (button.disabled) { return false; }
      button.disabled = true;
      accountMsg('<p><strong>' + t('signingIn') + '</strong></p>');
      post('/api/login', { email: form.email.value, password: form.password.value, mfa: form.mfa.value, accept: form.accept.value }, function (r) {
        button.disabled = false;
        form.mfa.value = '';
        if (r.ok) {
          form.password.value = '';
          accountMsg('<p><strong>' + escapeHtml(r.message) + '</strong></p>');
          setTimeout(function () { location.hash = r.next === 'vault' ? '#/vault' : '#/sync'; }, 1500);
          return;
        }
        accountMsg('<p class="alert"><strong>' + escapeHtml(r.message) + '</strong></p>');
        if (r.need2fa) { form.mfa.focus(); }
      }, function () {
        button.disabled = false;
        accountMsg('<p class="alert"><strong>' + t('noAnswer') + '</strong> ' + t('tryAgain') + '</p>');
      });
      return false;
    },
    accept: function () { // the third-party notice in the login form
      var a = document.getElementById('lf-accept');
      var input = a && a.parentNode.parentNode.getElementsByTagName('input')[0];
      if (!input) { return; }
      var on = input.value !== '1';
      input.value = on ? '1' : '';
      a.className = on ? 'btn on' : 'btn';
      a.innerHTML = escapeHtml(a.getAttribute(on ? 'data-on' : 'data-off'));
    },
    vault: function (id, askFirst) { // attach one of the account's vaults to this Kindle
      var box = document.getElementById('vaultmsg');
      if (!box || busy || !/^[A-Za-z0-9_-]+$/.test(id)) { return; }
      if (askFirst) {
        box.innerHTML = '<p class="alert">' + escapeHtml(box.getAttribute('data-confirm')) + '</p><p>' +
          btn('javascript:app.vault(\'' + id + '\',false)', escapeHtml(box.getAttribute('data-yes'))) + '</p>';
        window.scrollTo(0, document.body.scrollHeight);
        return;
      }
      var pw = document.getElementById('vp');
      busy = true;
      box.innerHTML = '<p><strong>' + escapeHtml(box.getAttribute('data-wait')) + '</strong></p>';
      window.scrollTo(0, document.body.scrollHeight);
      post('/api/setup', { vault: id, password: pw ? pw.value : '' }, function (r) {
        busy = false;
        if (r.ok) {
          if (pw) { pw.value = ''; }
          setMsg('vaultmsg', '<p><strong>' + escapeHtml(r.message) + '</strong> ' + t('syncingShort') + '</p>');
          setTimeout(function () { location.hash = '#/sync'; }, 1500);
          return;
        }
        setMsg('vaultmsg', '<p class="alert"><strong>' + escapeHtml(r.message) + '</strong></p>');
        if (r.needPassword && pw) { pw.focus(); }
      }, function () {
        busy = false;
        setMsg('vaultmsg', '<p class="alert"><strong>' + t('noAnswer') + '</strong> ' + t('tryAgain') + '</p>');
      }, 110000);
    },
    logout: function (confirmed) {
      if (!confirmed) {
        accountMsg('<p>' + t('logoutConfirm') + '</p><p>' + btn('javascript:app.logout(true)', t('logoutYes')) + '</p>');
        return;
      }
      accountMsg('<p>' + t('signingOut') + '</p>');
      post('/api/logout', {}, function (r) {
        if (r.ok) {
          show({ title: t('account'), html: '<div class="syncbox"><p><strong>' + escapeHtml(r.message) + '</strong></p></div><p>' +
            btn('#/account/login', t('signIn')) + ' ' + btn('#/', t('goHome')) + '</p>' });
        } else { accountMsg('<p class="alert"><strong>' + escapeHtml(r.message) + '</strong></p>'); }
      }, function () {
        accountMsg('<p class="alert"><strong>' + t('noAnswer') + '</strong></p>');
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
  drawMenu();
  initKindle();
  navigate();
})();
