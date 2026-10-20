// Idioma: el elegido antes o, si no, el del navegador. Sin almacenamiento disponible también funciona.
(function () {
  var botones = document.querySelectorAll('[data-idioma]');
  function aplicar(idioma) {
    document.documentElement.lang = idioma;
    // Los textos alternativos no admiten <span lang>: cada imagen guarda el inglés en data-alt-en.
    document.querySelectorAll('img[data-alt-en]').forEach(function (img) {
      if (!img.dataset.altEs) img.dataset.altEs = img.alt;
      img.alt = idioma === 'en' ? img.dataset.altEn : img.dataset.altEs;
    });
    botones.forEach(function (b) { b.setAttribute('aria-pressed', String(b.dataset.idioma === idioma)); });
  }
  var guardado = null;
  try { guardado = localStorage.getItem('tramtai.idioma'); } catch (e) {}
  aplicar(guardado || ((navigator.language || 'es').slice(0, 2) === 'es' ? 'es' : 'en'));
  botones.forEach(function (b) {
    b.addEventListener('click', function () {
      aplicar(b.dataset.idioma);
      try { localStorage.setItem('tramtai.idioma', b.dataset.idioma); } catch (e) {}
    });
  });
})();

// Tema: Sistema, Claro u Oscuro; el script de <head> ya aplicó el guardado antes de pintar.
(function () {
  var botones = document.querySelectorAll('button[data-tema]'); // <html> también lleva data-tema
  function aplicar(tema) {
    if (tema === 'sistema') delete document.documentElement.dataset.tema;
    else document.documentElement.dataset.tema = tema;
    botones.forEach(function (b) { b.setAttribute('aria-pressed', String(b.dataset.tema === tema)); });
  }
  var guardado = null;
  try { guardado = localStorage.getItem('tramtai.tema'); } catch (e) {}
  aplicar(guardado === 'claro' || guardado === 'oscuro' ? guardado : 'sistema');
  botones.forEach(function (b) {
    b.addEventListener('click', function () {
      aplicar(b.dataset.tema);
      try { localStorage.setItem('tramtai.tema', b.dataset.tema); } catch (e) {}
    });
  });
})();

// Botón «Copiar» en cada bloque de código (con los dos idiomas, como el resto de la página).
(function () {
  if (!navigator.clipboard) return;
  var copiar = '<span lang="es">Copiar</span><span lang="en">Copy</span>';
  var copiado = '<span lang="es">Copiado</span><span lang="en">Copied</span>';
  document.querySelectorAll('pre.codigo').forEach(function (pre) {
    var boton = document.createElement('button');
    boton.type = 'button';
    boton.className = 'copiar';
    boton.innerHTML = copiar;
    boton.addEventListener('click', function () {
      // El código que se ve ahora (un ejemplo cargado después), sin el texto del propio botón.
      var codigo = pre.querySelector('code');
      var texto = (codigo ? codigo.textContent : pre.textContent.replace(boton.textContent, '')).trim();
      navigator.clipboard.writeText(texto).then(function () {
        boton.innerHTML = copiado;
        setTimeout(function () { boton.innerHTML = copiar; }, 1500);
      });
    });
    pre.appendChild(boton);
  });
})();

// Índice de la ayuda: marca la sección que se está leyendo.
(function () {
  var enlaces = document.querySelectorAll('.indice a');
  if (!enlaces.length || !('IntersectionObserver' in window)) return;
  var observador = new IntersectionObserver(function (entradas) {
    entradas.forEach(function (e) {
      if (!e.isIntersecting) return;
      enlaces.forEach(function (a) { a.setAttribute('aria-current', String(a.getAttribute('href') === '#' + e.target.id)); });
    });
  }, { rootMargin: '-80px 0px -70% 0px' });
  document.querySelectorAll('.doc section[id]').forEach(function (s) { observador.observe(s); });
})();

// Descargas: enlaza cada botón con su archivo del último release (los nombres llevan la versión) y marca el sistema
// del visitante. Sin JavaScript o si GitHub no responde, los enlaces siguen llevando a la página del release.
(function () {
  if (!document.querySelector('[data-archivo]')) return;
  var nombres = {
    windows: '<span lang="es">Descargar para Windows</span><span lang="en">Download for Windows</span>',
    macos: '<span lang="es">Descargar para macOS</span><span lang="en">Download for macOS</span>',
    linux: '<span lang="es">Descargar para Linux</span><span lang="en">Download for Linux</span>'
  };
  var plataforma = (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || '';
  var movil = /android|iphone|ipad/i.test(navigator.userAgent);
  var sistema = movil ? '' : /win/i.test(plataforma) ? 'windows' : /mac/i.test(plataforma) ? 'macos' : /linux|x11|cros/i.test(plataforma) ? 'linux' : '';
  var tarjeta = sistema && document.querySelector('[data-sistema="' + sistema + '"]');
  if (tarjeta) {
    tarjeta.classList.add('tu-sistema');
    tarjeta.insertAdjacentHTML('afterbegin', '<span class="chapa"><span lang="es">Tu sistema</span><span lang="en">Your system</span></span>');
    document.querySelectorAll('[data-descarga] .texto-descarga').forEach(function (t) { t.innerHTML = nombres[sistema]; });
  }
  var principal = tarjeta && tarjeta.querySelector('a.boton[data-archivo]');

  function enlazar(archivos) {
    document.querySelectorAll('[data-archivo]').forEach(function (a) {
      var url = archivos[a.dataset.archivo];
      if (url) a.href = url;
    });
    if (principal && archivos[principal.dataset.archivo]) {
      document.querySelectorAll('[data-descarga]').forEach(function (a) { a.href = principal.href; });
    }
  }

  // En Linux con ARM (solo lo dicen los navegadores Chromium) el botón principal ofrece la versión ARM.
  function arquitectura() {
    if (sistema !== 'linux' || !navigator.userAgentData || !navigator.userAgentData.getHighEntropyValues) return Promise.resolve();
    return navigator.userAgentData.getHighEntropyValues(['architecture']).then(function (v) {
      if (v.architecture !== 'arm') return;
      document.querySelectorAll('[data-sistema="linux"] [data-arm]').forEach(function (a) { a.dataset.archivo = a.dataset.arm; });
      document.querySelectorAll('[data-sistema="linux"] [data-arq]').forEach(function (s) { s.textContent = '(ARM)'; });
      document.querySelectorAll('[data-sistema="linux"] [data-arq-otra]').forEach(function (s) { s.textContent = '(x64)'; });
    }).catch(function () {});
  }

  Promise.all([
    arquitectura(),
    fetch('https://api.github.com/repos/MscottB/Tramtai/releases/latest').then(function (r) { if (!r.ok) throw r; return r.json(); })
  ]).then(function (res) {
    var release = res[1];
    var archivos = {};
    (release.assets || []).forEach(function (f) {
      document.querySelectorAll('[data-archivo]').forEach(function (a) {
        if (f.name.slice(-a.dataset.archivo.length - 1) === '-' + a.dataset.archivo) archivos[a.dataset.archivo] = f.browser_download_url;
      });
    });
    enlazar(archivos);
    document.querySelectorAll('[data-version]').forEach(function (s) { s.textContent = ' · ' + String(release.tag_name).split('+')[0]; });
  }).catch(function () {});
})();

// Pestañas de lenguaje de la guía del desarrollador: todas cambian a la vez y se recuerda la elección. Cada panel carga
// su ejemplo de ejemplos/ (los de docs/ejemplos, que el CI ejecuta); sin JavaScript queda el enlace a GitHub.
(function () {
  var grupos = document.querySelectorAll('[data-pestanas]');
  if (!grupos.length) return;
  function elegir(lenguaje, foco) {
    grupos.forEach(function (g) {
      var botones = g.querySelectorAll('[role="tab"]');
      var hay = Array.prototype.some.call(botones, function (b) { return b.dataset.lenguaje === lenguaje; });
      botones.forEach(function (b, i) {
        var activo = hay ? b.dataset.lenguaje === lenguaje : i === 0;
        b.setAttribute('aria-selected', String(activo));
        b.tabIndex = activo ? 0 : -1;
        document.getElementById(b.getAttribute('aria-controls')).hidden = !activo;
        if (activo && foco === g) b.focus();
      });
    });
    try { localStorage.setItem('tramtai.lenguaje', lenguaje); } catch (e) {}
  }
  grupos.forEach(function (g) {
    var botones = Array.prototype.slice.call(g.querySelectorAll('[role="tab"]'));
    botones.forEach(function (b, i) {
      b.addEventListener('click', function () { elegir(b.dataset.lenguaje); });
      b.addEventListener('keydown', function (e) {
        var siguiente = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: botones.length - 1 }[e.key];
        if (siguiente === undefined) return;
        e.preventDefault();
        elegir(botones[(siguiente + botones.length) % botones.length].dataset.lenguaje, g);
      });
    });
  });
  var guardado = null;
  try { guardado = localStorage.getItem('tramtai.lenguaje'); } catch (e) {}
  elegir(guardado || 'python'); // sin JavaScript se ven todos los paneles, cada uno con su enlace a GitHub
  document.querySelectorAll('pre[data-ejemplo]').forEach(function (pre) {
    fetch('ejemplos/' + pre.dataset.ejemplo).then(function (r) {
      if (!r.ok) throw r;
      return r.text();
    }).then(function (texto) {
      pre.querySelector('code').textContent = texto.replace(/\s+$/, '');
    }).catch(function () {});
  });
})();
