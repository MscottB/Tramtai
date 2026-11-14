// bastanteVisto dice si un .vivo ya se ve lo bastante para arrancar: un cuarto de él o, si es muy alto, un cuarto de la
// pantalla. Sin margen recortado: un .vivo corto al final de la página también llega.
function bastanteVisto(e, alto) {
  return e.intersectionRatio >= 0.25 || e.intersectionRect.height >= alto * 0.25;
}

// Lo vivo (estilo.css) arranca su animación cuando entra en pantalla: .visto, una sola vez. Sin observador, ya. Va lo
// primero para que un fallo más abajo no deje nada esperando oculto. Este bloque solo ve los .vivo que ya están en la
// página: un .vivo creado después tiene que ponerse .visto él mismo.
(function () {
  var vivos = document.querySelectorAll('.vivo');
  if (!('IntersectionObserver' in window)) return vivos.forEach(function (v) { v.classList.add('visto'); });
  var vigia = new IntersectionObserver(function (entradas) {
    entradas.forEach(function (e) { if (bastanteVisto(e, innerHeight)) { e.target.classList.add('visto'); vigia.unobserve(e.target); } });
  }, { threshold: [0, 0.25] });
  vivos.forEach(function (v) { vigia.observe(v); });
})();

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
    // Lo mismo con las etiquetas para lectores de pantalla, los placeholder y el título de la pestaña.
    document.querySelectorAll('[data-aria-en]').forEach(function (e) {
      if (!e.dataset.ariaEs) e.dataset.ariaEs = e.getAttribute('aria-label');
      e.setAttribute('aria-label', idioma === 'en' ? e.dataset.ariaEn : e.dataset.ariaEs);
    });
    document.querySelectorAll('[data-placeholder-en]').forEach(function (e) {
      if (!e.dataset.placeholderEs) e.dataset.placeholderEs = e.placeholder;
      e.placeholder = idioma === 'en' ? e.dataset.placeholderEn : e.dataset.placeholderEs;
    });
    var titulo = document.querySelector('title[data-en]');
    if (titulo) {
      if (!titulo.dataset.es) titulo.dataset.es = titulo.textContent;
      document.title = idioma === 'en' ? titulo.dataset.en : titulo.dataset.es;
    }
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

// Menú de la barra en pantallas medianas y pequeñas: se abre con su botón y se cierra al elegir, con Escape o fuera.
(function () {
  var barra = document.querySelector('.barra');
  var boton = document.querySelector('.menu-boton');
  if (!barra || !boton) return;
  boton.hidden = false;
  function abrir(si) {
    barra.classList.toggle('abierta', si);
    boton.setAttribute('aria-expanded', String(si));
  }
  boton.addEventListener('click', function () { abrir(boton.getAttribute('aria-expanded') !== 'true'); });
  document.getElementById('menu').addEventListener('click', function (e) { if (e.target.closest('a, [data-buscar]')) abrir(false); });
  document.addEventListener('click', function (e) { if (!barra.contains(e.target)) abrir(false); });
  document.addEventListener('keydown', function (e) {
    if (e.key !== 'Escape' || !barra.classList.contains('abierta')) return;
    abrir(false);
    boton.focus();
  });
})();

// Botón «Copiar» en cada bloque de código (con los dos idiomas, como el resto de la página).
(function () {
  if (!navigator.clipboard) return;
  var copiar = '<span lang="es">Copiar</span><span lang="en">Copy</span>';
  var copiado = '<span lang="es">Copiado</span><span lang="en">Copied</span>';
  document.querySelectorAll('pre.codigo, pre.bloque-codigo').forEach(function (pre) {
    var boton = document.createElement('button');
    boton.type = 'button';
    boton.className = 'copiar';
    boton.innerHTML = copiar;
    boton.addEventListener('click', function () {
      // El código que se ve ahora (un ejemplo cargado después), sin el propio botón ni lo del otro idioma (comentarios,
      // o el <code> entero en la guía del desarrollador).
      var copia = pre.cloneNode(true);
      copia.querySelectorAll('.copiar, [lang]:not([lang="' + document.documentElement.lang + '"])').forEach(function (e) { e.remove(); });
      var texto = copia.textContent.trim();
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

// sistemaDe dice el sistema de escritorio del visitante por su navegador: 'windows', 'macos', 'linux' (x64) o null si no
// hay un paquete seguro para él (móvil, tableta, ChromeOS, Linux ARM o sin datos). Mejor «Descargar» a secas, que lleva
// a Descargar, que un archivo equivocado.
function sistemaDe(ua, plataforma) {
  var p = (plataforma || '') + ' ' + (ua || '');
  if (/android|iphone|ipad|ipod|mobile|cros/i.test(p)) return null;
  if (/\bwin(dows|32|64)\b/i.test(p)) return 'windows';
  if (/mac/i.test(p)) return 'macos';
  if (/linux|x11/i.test(p) && /x86_64|amd64/i.test(p) && !/aarch64|arm/i.test(p)) return 'linux';
  return null;
}

// destinoDescarga dice adónde lleva el botón [data-descarga]: al archivo del release (archivos, de #descargas) del
// primer [data-archivo] del bloque del sistema. En Linux no: .deb o .rpm depende de la distribución y se elige en
// Descargar. null deja el botón en descargar.html.
function destinoDescarga(sistema, archivo, archivos) {
  if (!sistema || sistema === 'linux' || !archivo) return null;
  return archivos[archivo] || null;
}

// Descargas: enlaza cada botón con su archivo del último release (los nombres llevan la versión) y marca el sistema
// del visitante. Los archivos y la versión los escribe la publicación del sitio en #descargas (web/scripts/descargas.mjs):
// ninguna visita consulta la API de GitHub. Sin JavaScript o sin esos datos, los enlaces llevan a la página del release.
// El botón [data-descarga] dice «Descargar para <sistema>» y lleva al archivo del primer [data-archivo] de su bloque
// [data-sistema] (destinoDescarga); sin sistema se queda en «Descargar», que lleva a Descargar.
(function () {
  if (!document.querySelector('[data-archivo]')) return;
  var nombres = {
    windows: '<span lang="es">Descargar para Windows</span><span lang="en">Download for Windows</span>',
    macos: '<span lang="es">Descargar para macOS</span><span lang="en">Download for macOS</span>',
    linux: '<span lang="es">Descargar para Linux</span><span lang="en">Download for Linux</span>'
  };
  var sistema = sistemaDe(navigator.userAgent, navigator.platform);
  // El iPad con Safari de escritorio se presenta como un Mac: con pantalla táctil, no lo es.
  if (sistema === 'macos' && navigator.maxTouchPoints > 1) sistema = null;
  // Inicio enseña solo cómo se instala en ese sistema (estilo.css).
  if (sistema) document.documentElement.dataset.tuSistema = sistema;
  // El sistema de escritorio aunque sea ARM: Descargar abre su pestaña, que enseña los dos paquetes.
  var tuya = sistema;
  var datos = {};
  try { datos = JSON.parse(document.getElementById('descargas').textContent); } catch (e) {}

  function enlazar() {
    var archivos = datos.archivos || {};
    document.querySelectorAll('[data-archivo]').forEach(function (a) {
      var url = archivos[a.dataset.archivo];
      if (url) a.href = url;
    });
    // Las pestañas de Descargar (el código de más abajo las prepara antes de que esto corra): abre la de tu sistema.
    var pestana = tuya && document.querySelector('[data-pestanas="descargar"] [role="tab"][data-lenguaje="' + tuya + '"]');
    if (pestana) pestana.click();
    var bloque = sistema && document.querySelector('[data-sistema="' + sistema + '"]');
    if (!bloque) return;
    document.querySelectorAll('[data-descarga] .texto-descarga').forEach(function (t) { t.innerHTML = nombres[sistema]; });
    document.querySelectorAll('article[data-sistema="' + sistema + '"]').forEach(function (t) {
      t.classList.add('tu-sistema');
      t.insertAdjacentHTML('afterbegin', '<span class="chapa"><span lang="es">Tu sistema</span><span lang="en">Your system</span></span>');
    });
    var principal = bloque.querySelector('[data-archivo]');
    var destino = destinoDescarga(sistema, principal && principal.dataset.archivo, archivos);
    if (destino) document.querySelectorAll('[data-descarga]').forEach(function (a) { a.href = destino; });
  }

  // Linux con ARM (solo lo dicen los navegadores Chromium, aunque su texto diga x86_64): Descargar ofrece los paquetes
  // ARM y el botón se queda en «Descargar».
  function arquitectura() {
    if (sistema !== 'linux' || !navigator.userAgentData || !navigator.userAgentData.getHighEntropyValues) return Promise.resolve();
    return navigator.userAgentData.getHighEntropyValues(['architecture']).then(function (v) {
      if (v.architecture !== 'arm') return;
      sistema = null;
      delete document.documentElement.dataset.tuSistema;
      document.querySelectorAll('[data-sistema="linux"] [data-arm]').forEach(function (a) { a.dataset.archivo = a.dataset.arm; });
      document.querySelectorAll('[data-sistema="linux"] [data-arq]').forEach(function (s) { s.textContent = '(ARM)'; });
      document.querySelectorAll('[data-sistema="linux"] [data-arq-otra]').forEach(function (s) { s.textContent = '(x64)'; });
    }).catch(function () {});
  }

  arquitectura().then(enlazar);
})();

// Pestañas de lenguaje de la guía del desarrollador: todas cambian a la vez y se recuerda la elección. Cada panel carga
// su ejemplo de ejemplos/ en cada idioma (los de docs/ejemplos, que el CI ejecuta); sin JavaScript queda el enlace a GitHub.
(function () {
  var grupos = document.querySelectorAll('[data-pestanas]');
  if (!grupos.length) return;
  // La Ayuda recuerda su lenguaje aparte; el resto del sitio (la guía, «Cómo funciona» en Producto) comparte uno. Si el
  // guardado no está en la página, sale la primera pestaña.
  var clave = document.querySelector('.tema-ayuda') ? 'tramtai.lenguaje-ayuda' : 'tramtai.lenguaje';
  // Cada data-pestanas="nombre" es un juego aparte (el de lenguajes no tiene nombre; Linux · Windows Server es
  // «sistema»): sus pestañas cambian a la vez y recuerdan su elección sin tocar las del otro juego.
  function juego(lenguaje) {
    var b = document.querySelector('[data-pestanas] [role="tab"][data-lenguaje="' + CSS.escape(lenguaje) + '"]');
    return b ? b.closest('[data-pestanas]').dataset.pestanas : '';
  }
  // pestanasDe da las pestañas del juego, las de su primera lista: un panel puede llevar dentro otras (la terminal de
  // Inicio, en la ventana de cuatro formas), que no son suyas.
  function pestanasDe(g) { return g.querySelector('[role="tablist"]').querySelectorAll('[role="tab"]'); }
  // elegir enseña la pestaña; solo se recuerda la que elige la persona (guardar), no la de por defecto al cargar.
  function elegir(lenguaje, foco, guardar) {
    var nombre = juego(lenguaje);
    grupos.forEach(function (g) {
      if ((g.dataset.pestanas || '') !== nombre) return;
      var botones = pestanasDe(g);
      var hay = Array.prototype.some.call(botones, function (b) { return b.dataset.lenguaje === lenguaje; });
      botones.forEach(function (b, i) {
        var activo = hay ? b.dataset.lenguaje === lenguaje : i === 0;
        b.setAttribute('aria-selected', String(activo));
        b.tabIndex = activo ? 0 : -1;
        document.getElementById(b.getAttribute('aria-controls')).hidden = !activo;
        if (activo && foco === g) b.focus();
      });
    });
    // El sistema elegido vale en la portada y en servidor.html. La ventana de Inicio siempre abre en su primera forma.
    if (guardar && nombre !== 'formas' && nombre !== 'descargar') try { localStorage.setItem(nombre ? 'tramtai.' + nombre : clave, lenguaje); } catch (e) {}
  }
  grupos.forEach(function (g) {
    var botones = Array.prototype.slice.call(pestanasDe(g));
    // En la portada, sin JavaScript, las pestañas esperan ocultas y se ve el primer ejemplo: aquí se encienden.
    var lista = g.querySelector('[role="tablist"]');
    lista.hidden = false;
    botones.forEach(function (b, i) {
      document.getElementById(b.getAttribute('aria-controls')).setAttribute('role', 'tabpanel');
      b.addEventListener('click', function () { elegir(b.dataset.lenguaje, null, true); });
      b.addEventListener('keydown', function (e) {
        var siguiente = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: botones.length - 1 }[e.key];
        if (siguiente === undefined) return;
        e.preventDefault();
        elegir(botones[(siguiente + botones.length) % botones.length].dataset.lenguaje, g, true);
      });
    });
  });
  var guardado = null;
  try { guardado = localStorage.getItem(clave); } catch (e) {}
  // Sin nada guardado, Python donde lo hay y, si no, la primera pestaña (OpenAI en «Cómo funciona»). Sin JavaScript se ven
  // todos los paneles de la guía, cada uno con su enlace a GitHub.
  elegir(guardado || 'python');
  // Cada juego con nombre (Linux · Windows Server es «sistema»; las cuatro formas de Inicio, «formas», que no se
  // recuerda): el guardado, si sigue siendo de ese juego, o su primera pestaña. Sin JavaScript se ven todos sus paneles.
  document.querySelectorAll('[data-pestanas]:not([data-pestanas=""])').forEach(function (g) {
    var nombre = g.dataset.pestanas, elegido = null;
    if (nombre !== 'formas' && nombre !== 'descargar') try { elegido = localStorage.getItem('tramtai.' + nombre); } catch (e) {}
    elegir(elegido && juego(elegido) === nombre ? elegido : pestanasDe(g)[0].dataset.lenguaje);
  });
  // Cada panel lleva un <code lang="es"> y uno lang="en" (los de docs/ejemplos/*/en): el CSS enseña el del idioma
  // elegido, y Copiar solo copia ese.
  document.querySelectorAll('code[data-ejemplo]').forEach(function (code) {
    fetch('ejemplos/' + code.dataset.ejemplo).then(function (r) {
      if (!r.ok) throw r;
      return r.text();
    }).then(function (texto) {
      code.textContent = texto.replace(/\s+$/, '');
    }).catch(function () {});
  });
})();

// Pestañas de la terminal de la portada (Inicio, Mis APIs, Historial), con el patrón de las demás: ratón, flechas, Inicio
// y Fin, y además la tecla de cada página en la TUI (1, 4, 5). Las teclas solo cuentan con el foco en las pestañas, para
// no quitarle atajos a la página. Sin JavaScript, Inicio se ve y las otras están en <details>: aquí salen de ahí.
(function () {
  var ventana = document.querySelector('.ventana-terminal');
  if (!ventana) return;
  var lista = ventana.querySelector('[role="tablist"]');
  var botones = Array.prototype.slice.call(lista.querySelectorAll('[role="tab"]'));
  var paneles = botones.map(function (b) {
    var panel = document.getElementById(b.getAttribute('aria-controls'));
    var detalles = panel.closest('details');
    if (detalles) detalles.replaceWith(panel);
    panel.setAttribute('role', 'tabpanel');
    return panel;
  });
  function elegir(n, foco) {
    botones.forEach(function (b, i) {
      b.setAttribute('aria-selected', String(i === n));
      b.tabIndex = i === n ? 0 : -1;
      paneles[i].hidden = i !== n;
    });
    if (foco) botones[n].focus();
  }
  botones.forEach(function (b, i) { b.addEventListener('click', function () { elegir(i); }); });
  lista.addEventListener('keydown', function (e) {
    if (e.ctrlKey || e.altKey || e.metaKey) return;
    var i = botones.indexOf(document.activeElement);
    var n = { ArrowRight: i + 1, ArrowLeft: i - 1, Home: 0, End: botones.length - 1 }[e.key];
    if (n === undefined) {
      n = botones.map(function (b) { return b.dataset.tecla; }).indexOf(e.key);
      if (n < 0) return;
    }
    e.preventDefault();
    elegir((n + botones.length) % botones.length, true);
  });
  lista.hidden = false;
  elegir(0);
})();

// Buscador del sitio: Ctrl K, ⌘K, / o un botón con data-buscar (la barra, el campo de la Ayuda) abren un diálogo que busca
// por palabras en buscar.json, en el idioma que se ve. Las flechas mueven, Enter abre, Esc cierra y el foco vuelve a quien
// lo abrió. Sin JavaScript los botones siguen ocultos y se navega por la barra y las tarjetas.
(function () {
  var botones = document.querySelectorAll('[data-buscar]');
  if (!botones.length || typeof HTMLDialogElement !== 'function') return;
  // buscar.json está en la raíz del sitio, junto a este script, desde cualquier página.
  var script = document.currentScript || document.querySelector('script[src*="sitio.js"]');
  var base = script ? new URL('.', script.src).href : '';
  var dialogo, campo, lista, cuenta, indice = null, cargando = null, opciones = [], activo = -1, quien = null;
  function lengua() { return document.documentElement.lang === 'en' ? 'en' : 'es'; }
  function tx(es, en) { return lengua() === 'en' ? en : es; }
  function normalizar(s) {
    return s.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase().replace(/[\s_]+/g, ' ').trim();
  }
  // raiz es la de web/src/ayuda/buscar.ts: «filtrada» encuentra «se filtra».
  function raiz(p) {
    if (p.length <= 5) return p;
    var corta = p.replace(/(aciones|acion|adas|ados|ada|ado|idas|idos|ida|ido|es|s)$/, '');
    return corta.length >= 4 ? corta : p;
  }
  function cargar() {
    if (!cargando) cargando = fetch(base + 'buscar.json').then(function (r) { if (!r.ok) throw r; return r.json(); }).then(function (datos) {
      ['es', 'en'].forEach(function (l) { datos[l].forEach(function (e) { e.n = normalizar(e.ti + ' ' + e.d); }); });
      indice = datos;
    }).catch(function () { cargando = null; });
    return cargando;
  }
  function mover(n) {
    if (!opciones.length) return;
    activo = (n + opciones.length) % opciones.length;
    opciones.forEach(function (o, i) { o.setAttribute('aria-selected', String(i === activo)); });
    campo.setAttribute('aria-activedescendant', opciones[activo].id);
    opciones[activo].scrollIntoView({ block: 'nearest' });
  }
  function buscar() {
    var palabras = normalizar(campo.value).split(' ').filter(Boolean).map(raiz);
    lista.textContent = '';
    opciones = [];
    activo = -1;
    cuenta.textContent = '';
    campo.removeAttribute('aria-activedescendant');
    campo.setAttribute('aria-expanded', 'false');
    if (!palabras.length) return;
    if (!indice) { if (!cargando) cuenta.textContent = tx('No se pudo cargar el buscador.', 'The search could not load.'); return; }
    var coincide = function (e, p) { return e.x.indexOf(p) >= 0 || e.n.indexOf(p) >= 0; };
    var hallados = indice[lengua()].filter(function (e) {
      return palabras.every(function (p) { return coincide(e, p); });
    }).sort(function (a, b) {
      var pa = palabras.every(function (p) { return a.n.indexOf(p) >= 0; }) ? 0 : 1;
      var pb = palabras.every(function (p) { return b.n.indexOf(p) >= 0; }) ? 0 : 1;
      return pa - pb;
    }).slice(0, 30);
    hallados.forEach(function (e, i) {
      // Un enlace real: Ctrl+clic, clic central y lectores de pantalla lo tratan como tal.
      var li = document.createElement('a');
      li.href = base + e.u;
      li.tabIndex = -1;
      li.id = 'buscador-' + i;
      li.setAttribute('role', 'option');
      li.setAttribute('aria-selected', 'false');
      var ti = document.createElement('span');
      ti.textContent = e.ti;
      var d = document.createElement('span');
      d.className = 'b-d';
      d.textContent = e.d;
      li.append(ti, d);
      li.addEventListener('mousedown', function (ev) { ev.preventDefault(); }); // el foco se queda en el campo
      // Con un clic normal el diálogo se cierra y el navegador sigue el enlace; con Ctrl, ⌘ o Mayús se abre aparte y queda.
      li.addEventListener('click', function (ev) { if (!(ev.ctrlKey || ev.metaKey || ev.shiftKey || ev.altKey)) dialogo.close(); });
      lista.appendChild(li);
      opciones.push(li);
    });
    var en = lengua() === 'en';
    cuenta.textContent = hallados.length
      ? hallados.length + (en ? (hallados.length === 1 ? ' result' : ' results') : (hallados.length === 1 ? ' resultado' : ' resultados'))
      : (en ? 'Nothing matches “' : 'Nada coincide con «') + campo.value.trim() + (en ? '”.' : '».');
    if (hallados.length) { campo.setAttribute('aria-expanded', 'true'); mover(0); }
  }
  function crear() {
    dialogo = document.createElement('dialog');
    dialogo.className = 'buscador';
    dialogo.setAttribute('aria-labelledby', 'buscador-titulo');
    dialogo.innerHTML = '<div class="buscador-caja">' +
      '<h2 id="buscador-titulo" class="solo-lectores"><span lang="es">Buscar en el sitio</span><span lang="en">Search the site</span></h2>' +
      '<input type="text" role="combobox" aria-controls="buscador-lista" aria-expanded="false" aria-autocomplete="list" autocomplete="off" spellcheck="false" enterkeyhint="search">' +
      '<p id="buscador-cuenta" class="buscador-cuenta" aria-live="polite"></p>' +
      '<div id="buscador-lista" role="listbox"></div>' +
      '<p class="buscador-pie" aria-hidden="true"><span lang="es"><kbd>↑</kbd> <kbd>↓</kbd> para moverte, <kbd>Enter</kbd> para abrir, <kbd>Esc</kbd> para cerrar</span><span lang="en"><kbd>↑</kbd> <kbd>↓</kbd> to move, <kbd>Enter</kbd> to open, <kbd>Esc</kbd> to close</span></p></div>';
    document.body.appendChild(dialogo);
    campo = dialogo.querySelector('input');
    lista = dialogo.querySelector('[role="listbox"]');
    cuenta = dialogo.querySelector('.buscador-cuenta');
    campo.addEventListener('input', buscar);
    campo.addEventListener('keydown', function (e) {
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') { e.preventDefault(); mover(activo + (e.key === 'ArrowDown' ? 1 : -1)); }
      // Ctrl+Enter o ⌘+Enter abre el resultado en otra pestaña, como Ctrl+clic: el clic lleva las teclas.
      else if (e.key === 'Enter' && activo >= 0) {
        e.preventDefault();
        opciones[activo].dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: window, ctrlKey: e.ctrlKey, metaKey: e.metaKey, shiftKey: e.shiftKey }));
      }
    });
    // Clic en el fondo (el propio diálogo, sin su caja) cierra.
    dialogo.addEventListener('click', function (e) { if (e.target === dialogo) dialogo.close(); });
    // El foco vuelve a quien lo abrió; si era el botón del menú de la barra, ya cerrado, vuelve al botón del menú.
    dialogo.addEventListener('close', function () {
      var menu = document.querySelector('.menu-boton:not([hidden])');
      var f = quien && quien.offsetParent ? quien : menu && menu.offsetParent ? menu : document.body;
      quien = null;
      f.focus();
    });
  }
  function abrir() {
    if (!dialogo) crear();
    if (dialogo.open) return;
    quien = document.activeElement;
    var rotulo = tx('Buscar en el sitio', 'Search the site');
    campo.placeholder = rotulo;
    campo.setAttribute('aria-label', rotulo);
    lista.setAttribute('aria-label', tx('Resultados', 'Results'));
    campo.value = '';
    buscar();
    dialogo.showModal();
    campo.focus();
    cargar().then(function () { if (dialogo.open) buscar(); });
  }
  botones.forEach(function (b) {
    b.hidden = false;
    b.setAttribute('aria-keyshortcuts', 'Control+K Meta+K /');
    b.addEventListener('click', abrir);
  });
  document.addEventListener('keydown', function (e) {
    var t = e.target;
    var escribiendo = t && (/^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName) || t.isContentEditable);
    if ((e.ctrlKey || e.metaKey) && !e.altKey && !e.shiftKey && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      if (dialogo && dialogo.open) dialogo.close(); else abrir();
    } else if (e.key === '/' && !e.ctrlKey && !e.metaKey && !e.altKey && !escribiendo) {
      e.preventDefault();
      abrir();
    }
  });
})();
// Los desplegables (<details class="desplegable">: los otros sistemas junto a Descargar, en Inicio) se cierran con Escape
// o al pulsar fuera.
(function () {
  var menus = document.querySelectorAll('details.desplegable');
  if (!menus.length) return;
  document.addEventListener('click', function (e) {
    menus.forEach(function (d) { if (d.open && !d.contains(e.target)) d.open = false; });
  });
  document.addEventListener('keydown', function (e) {
    if (e.key !== 'Escape') return;
    menus.forEach(function (d) { if (d.open) { d.open = false; d.querySelector('summary').focus(); } });
  });
})();

// Al llegar a una pregunta de la Ayuda por su ancla (#terminos), también desde un enlace de la misma página, se abre.
(function () {
  function abrir() {
    var destino = location.hash && document.getElementById(location.hash.slice(1));
    if (destino && destino.tagName === 'DETAILS') destino.open = true;
  }
  abrir();
  window.addEventListener('hashchange', abrir);
})();

// escribirLeyenda cambia el texto de la leyenda del tranvía o de la demo un momento después de fijar su aria-live
// (anunciar): algunos lectores no anuncian el cambio que llega a la vez que el atributo.
function escribirLeyenda(leyenda, partes) {
  clearTimeout(leyenda.pendiente);
  leyenda.pendiente = setTimeout(function () {
    leyenda.innerHTML = '';
    partes.forEach(function (p) { leyenda.appendChild(p); });
  }, 100);
}

// Demo del recorrido guiado (en Producto): la zona de la app que señala cada paso de la bienvenida, un paso cada 6 s:
// pausa que cambia su texto, la leyenda solo se anuncia cuando elige la persona,
// quieta con «reducir movimiento» y sin avanzar con la pestaña oculta, fuera de la pantalla ni con el foco dentro. La
// lista de pasos (un desplegable en el móvil) y las flechas eligen el paso; ← → solo con el foco dentro de la demo. Sin
// JavaScript se ven todos los pasos, uno debajo de otro, con su texto.
(function () {
  var caja = document.querySelector('.demo');
  var controles = caja && caja.querySelector('.demo-controles');
  if (!controles) return;
  var figuras = caja.querySelectorAll('.demo-paso');
  var botones = caja.querySelectorAll('.demo-ir');
  var anterior = caja.querySelector('.demo-anterior');
  var siguiente = caja.querySelector('.demo-siguiente');
  var progreso = caja.querySelector('.demo-progreso span');
  var detalles = caja.querySelector('.demo-pasos-movil');
  var leyenda = caja.querySelector('.demo-leyenda');
  var pausa = caja.querySelector('.demo-pausa');
  var movimiento = window.matchMedia('(prefers-reduced-motion: reduce)');
  var movil = window.matchMedia('(max-width: 899px)');
  var CADA_MS = 6000;
  // pausaManual es la pausa que eligió la persona: «reducir movimiento» no la pisa al volver a desactivarse.
  // foco: el foco está dentro de la demo; el avance solo se para mientras tanto (patrón de carrusel), sin contar como pausa.
  var actual = 0, pausado = movimiento.matches, pausaManual = false, foco = false, enPantalla = !('IntersectionObserver' in window), temporizador = null;

  function mostrar(n) {
    actual = n;
    figuras.forEach(function (f, i) { f.hidden = i !== n; });
    botones.forEach(function (b, i) { if (i === n) b.setAttribute('aria-current', 'step'); else b.removeAttribute('aria-current'); });
    // La flecha que no lleva a ninguna parte se desactiva.
    anterior.disabled = n === 0;
    siguiente.disabled = n === figuras.length - 1;
    progreso.style.width = (n + 1) / figuras.length * 100 + '%';
    escribirLeyenda(leyenda, ['es', 'en'].map(function (idioma) {
      var s = document.createElement('span');
      s.lang = idioma;
      s.textContent = [figuras[n].querySelector('.demo-donde'), figuras[n].querySelector('h3')].map(function (e) {
        return e.querySelector('[lang="' + idioma + '"]').textContent;
      }).join(' · ');
      return s;
    }));
  }
  // precargar trae la imagen del paso siguiente que se va a ver (la del tema y el idioma activos, no las cuatro), así no
  // sale en blanco al avanzar: las de los pasos ocultos tienen carga diferida y no bajan hasta que se ven.
  function precargar(n) {
    var vista = [].filter.call(figuras[n].querySelectorAll('img'), function (i) { return i.offsetWidth > 0; })[0];
    var siguiente = vista && figuras[(n + 1) % figuras.length].querySelector('img.' + vista.classList[0] + '[lang="' + vista.lang + '"]');
    if (siguiente) siguiente.loading = 'eager';
  }
  // La leyenda se anuncia cuando la elige la persona, no cada vez que avanza sola.
  function anunciar(si) { leyenda.setAttribute('aria-live', si ? 'polite' : 'off'); }
  function programar() {
    clearTimeout(temporizador);
    if (pausado || foco || !enPantalla || document.hidden) return;
    precargar(actual);
    temporizador = setTimeout(function () {
      anunciar(false);
      mostrar((actual + 1) % figuras.length);
      programar();
    }, CADA_MS);
  }
  function rotulo() {
    pausa.innerHTML = pausado ? '<span lang="es">Seguir</span><span lang="en">Play</span>' : '<span lang="es">Pausa</span><span lang="en">Pause</span>';
  }

  // Solo lo que elige la persona mueve el foco: si la flecha que lo tenía se desactiva, se lo pasa a la otra.
  function elegir(n) {
    var conFoco = document.activeElement;
    anunciar(true);
    mostrar(n);
    if (conFoco === anterior && anterior.disabled) siguiente.focus();
    if (conFoco === siguiente && siguiente.disabled) anterior.focus();
    programar();
  }
  botones.forEach(function (b, i) {
    b.addEventListener('click', function () {
      elegir(i);
      if (!movil.matches) return;
      // En el móvil el desplegable se cierra y deja ver el paso; el foco, que estaba en la lista, vuelve a su rótulo.
      detalles.open = false;
      detalles.querySelector('summary').focus();
    });
  });
  // En el escritorio la lista va fija al lado: su rótulo no la pliega.
  detalles.querySelector('summary').addEventListener('click', function (e) { if (!movil.matches) e.preventDefault(); });
  anterior.addEventListener('click', function () { elegir(actual - 1); });
  siguiente.addEventListener('click', function () { elegir(actual + 1); });
  caja.addEventListener('focusin', function () { foco = true; programar(); });
  caja.addEventListener('focusout', function (e) {
    if (caja.contains(e.relatedTarget)) return;
    foco = false;
    programar();
  });
  caja.addEventListener('keydown', function (e) {
    if (e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return; // Alt+← es el atrás del navegador
    var n = actual + (e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0);
    if (n === actual || n < 0 || n >= figuras.length) return;
    e.preventDefault();
    elegir(n);
  });
  // La lista va abierta al lado en el escritorio y cerrada encima en el móvil.
  function desplegable() { detalles.open = !movil.matches; }
  movil.addEventListener('change', desplegable);
  desplegable();
  pausa.addEventListener('click', function () {
    pausado = !pausado;
    pausaManual = pausado;
    // «Seguir» es querer el avance: el foco, que sigue en la pausa, no lo para hasta moverse por la demo.
    if (!pausado) foco = false;
    rotulo();
    programar();
  });
  document.addEventListener('visibilitychange', programar);
  if ('IntersectionObserver' in window) {
    new IntersectionObserver(function (entradas) {
      enPantalla = entradas[entradas.length - 1].isIntersecting;
      programar();
    }, { threshold: 0.25 }).observe(caja);
  }
  // «Reducir movimiento» se respeta también si cambia con la página abierta.
  movimiento.addEventListener('change', function () {
    pausado = movimiento.matches || pausaManual;
    pausa.hidden = movimiento.matches;
    rotulo();
    programar();
  });
  pausa.hidden = pausado;
  rotulo();
  anterior.hidden = siguiente.hidden = controles.hidden = false;
  mostrar(0);
  programar();
})();


// Plano de metro de Funciones: sin JavaScript, o por debajo de 900 px, cada línea va en vertical con el texto de sus
// paradas, y el título de cada una es texto. Con pantalla ancha va en horizontal (metro--plano): el título pasa a ser un
// botón que enseña su título y su texto en el panel de debajo, y la elegida queda marcada con aria-current. Como las
// pestañas de la cabecera, solo la elegida está en el tabulador y las flechas pasan de una a otra. Empieza elegida la
// primera.
(function () {
  var metro = document.querySelector('.metro');
  var panel = metro && metro.querySelector('.metro-panel');
  if (!panel) return;
  var titulos = Array.prototype.slice.call(metro.querySelectorAll('.parada-titulo'));
  var ancho = window.matchMedia('(min-width: 900px)');
  var elegida = 0, botones = [];
  function elegir(n, foco) {
    elegida = n;
    botones.forEach(function (b, i) {
      b.tabIndex = i === n ? 0 : -1;
      if (i === n) b.setAttribute('aria-current', 'true');
      else b.removeAttribute('aria-current');
    });
    if (foco) botones[n].focus();
    // El título del panel es un párrafo: un h4 más saldría en el esquema de títulos colgado de la última línea.
    panel.innerHTML = '<p class="metro-panel-titulo">' + botones[n].innerHTML + '</p><p>' + titulos[n].closest('.parada').querySelector('.parada-texto').innerHTML + '</p>';
  }
  function boton(titulo, i) {
    var b = document.createElement('button');
    b.type = 'button';
    b.className = 'parada-boton';
    b.append.apply(b, Array.prototype.slice.call(titulo.childNodes));
    titulo.appendChild(b);
    b.addEventListener('click', function () { elegir(i); });
    b.addEventListener('keydown', function (e) {
      var siguiente = { ArrowRight: i + 1, ArrowDown: i + 1, ArrowLeft: i - 1, ArrowUp: i - 1, Home: 0, End: titulos.length - 1 }[e.key];
      if (siguiente === undefined || e.altKey || e.ctrlKey || e.metaKey) return;
      e.preventDefault();
      elegir((siguiente + titulos.length) % titulos.length, true);
    });
    return b;
  }
  // En vertical el texto ya se ve: el botón vuelve a ser el texto del título, sin un botón que no haría nada.
  function modo() {
    var plano = ancho.matches;
    metro.classList.toggle('metro--plano', plano);
    if (plano && !botones.length) botones = titulos.map(boton);
    if (!plano) {
      botones.forEach(function (b) { b.replaceWith.apply(b, Array.prototype.slice.call(b.childNodes)); });
      botones = [];
    }
    if (plano) elegir(elegida); // con el panel aún oculto: el texto inicial no se anuncia
    panel.hidden = !plano;
  }
  ancho.addEventListener('change', modo);
  modo();
})();
