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
  document.getElementById('menu').addEventListener('click', function (e) { if (e.target.closest('a')) abrir(false); });
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
  // La Ayuda tiene otros lenguajes que la guía: cada una recuerda el suyo.
  var clave = document.querySelector('.tema-ayuda') ? 'tramtai.lenguaje-ayuda' : 'tramtai.lenguaje';
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
    try { localStorage.setItem(clave, lenguaje); } catch (e) {}
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
  try { guardado = localStorage.getItem(clave); } catch (e) {}
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

// Buscador de la Ayuda (site/ayuda/): busca por palabras en indice.json, en el idioma que se ve. Sin JavaScript queda
// oculto y se navega por las tarjetas.
(function () {
  var caja = document.querySelector('.buscar-ayuda');
  var campo = document.getElementById('buscar-ayuda');
  var lista = document.getElementById('resultados-ayuda');
  if (!caja || !campo || !lista) return;
  var indice = null;
  caja.hidden = false;
  function normalizar(s) {
    return s.normalize('NFD').replace(/[\u0300-\u036f]/g, '').toLowerCase().replace(/[\s_]+/g, ' ').trim();
  }
  // raiz es la de web/src/ayuda/buscar.ts: «filtrada» encuentra «se filtra».
  function raiz(p) {
    if (p.length <= 5) return p;
    var corta = p.replace(/(aciones|acion|adas|ados|ada|ado|idas|idos|ida|ido|es|s)$/, '');
    return corta.length >= 4 ? corta : p;
  }
  function mostrar() {
    var palabras = normalizar(campo.value).split(' ').filter(Boolean).map(raiz);
    var cuenta = document.getElementById('cuenta-ayuda');
    lista.innerHTML = '';
    if (cuenta) cuenta.textContent = '';
    if (!palabras.length || !indice) return;
    var entradas = indice[document.documentElement.lang === 'en' ? 'en' : 'es'] || [];
    var hallados = entradas.filter(function (e) {
      return palabras.every(function (p) { return e.x.indexOf(p) >= 0; });
    }).sort(function (a, b) {
      var pa = palabras.every(function (p) { return normalizar(a.d).indexOf(p) >= 0; }) ? 0 : 1;
      var pb = palabras.every(function (p) { return normalizar(b.d).indexOf(p) >= 0; }) ? 0 : 1;
      return pa - pb;
    }).slice(0, 30);
    hallados.forEach(function (e) {
      var li = document.createElement('li');
      var a = document.createElement('a');
      a.href = e.t + '.html' + (e.a ? '#' + e.a : '');
      a.textContent = e.d;
      if (e.d !== e.ti) {
        var donde = document.createElement('span');
        donde.textContent = e.ti;
        a.appendChild(donde);
      }
      li.appendChild(a);
      lista.appendChild(li);
    });
    var en = document.documentElement.lang === 'en';
    if (cuenta) cuenta.textContent = hallados.length + (en ? (hallados.length === 1 ? ' result' : ' results') : (hallados.length === 1 ? ' resultado' : ' resultados'));
    if (!hallados.length) {
      var vacio = document.createElement('li');
      vacio.textContent = (en ? 'Nothing matches “' : 'Nada coincide con «') + campo.value.trim() + (en ? '”.' : '».');
      lista.appendChild(vacio);
    }
  }
  fetch('indice.json').then(function (r) { if (!r.ok) throw r; return r.json(); }).then(function (datos) {
    indice = datos;
    mostrar();
  }).catch(function () { caja.hidden = true; });
  campo.addEventListener('input', mostrar);
  // Desde «Buscar» de cada tema se llega con el campo listo para escribir.
  if (location.hash === '#buscar-ayuda') campo.focus();
  // Al cambiar de idioma, los resultados se rehacen en el nuevo.
  document.querySelectorAll('[data-idioma]').forEach(function (b) { b.addEventListener('click', function () { setTimeout(mostrar); }); });
})();

// Al llegar a una pregunta de la Ayuda por su ancla (#terminos), se abre.
(function () {
  var destino = location.hash && document.getElementById(location.hash.slice(1));
  if (destino && destino.tagName === 'DETAILS') destino.open = true;
})();

// Recorrido de la portada: el tranvía va de una app a Tramtai y sigue por la línea de un agente hasta su estación, con
// cuatro ejemplos que se turnan (uno, el respaldo: Claude falla a mitad y sigue Codex). La leyenda y el código de
// «Cómo funciona» cambian con cada uno. Pausa, puntos para elegir, y sin animación con «reducir movimiento».
(function () {
  var datos = document.getElementById('ejemplos-mapa');
  var caja = document.querySelector('.mapa-ejemplos');
  var ancho = document.querySelector('.mapa-ancho');
  var alto = document.querySelector('.mapa-alto');
  if (!datos || !caja || !ancho || !alto || !Element.prototype.animate) return;
  var ejemplos = JSON.parse(datos.textContent);
  var movimiento = window.matchMedia('(prefers-reduced-motion: reduce)');
  var quieto = movimiento.matches;
  var ORIGENES = ['M 190 95 L 202 95 C 240 95 240 143 278 143 L 290 143', 'M 190 153 L 204 153 C 240 153 240 153 276 153 L 290 153',
    'M 190 211 L 202 211 C 240 211 240 163 278 163 L 290 163'];
  var LINEAS = ['M 460 133 L 472 133 C 515 133 515 37 558 37 L 570 37', 'M 460 143 L 472 143 C 515 143 515 95 558 95 L 570 95',
    'M 460 153 L 479 153 C 515 153 515 153 551 153 L 570 153', 'M 460 163 L 472 163 C 515 163 515 211 558 211 L 570 211',
    'M 460 173 L 472 173 C 515 173 515 269 558 269 L 570 269'];
  var ALTO_ORIGEN = [14, 52, 90];
  var ALTO_LINEA = [210, 248, 286, 324, 362];
  var ROJO = '#d64545';
  var trams = [ancho.querySelector('.tranvia'), alto.querySelector('.tranvia-alto')];
  trams.forEach(function (t) { t.style.animation = 'none'; t.style.opacity = '0'; });

  // largo mide un camino con un path suelto, fuera de la página: meterlo en el svg obligaba al navegador a recalcular
  // estilos y maquetar todo (reflow forzado). Si algún navegador no mide un path suelto, se mide dentro del svg.
  // Cada camino se mide una sola vez.
  var medidas = {};
  function largo(svg, d) {
    if (medidas[d] !== undefined) return medidas[d];
    var p = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    p.setAttribute('d', d);
    var l = 0;
    try { l = p.getTotalLength(); } catch (e) {}
    if (!l) {
      svg.appendChild(p);
      l = p.getTotalLength();
      svg.removeChild(p);
    }
    medidas[d] = l;
    return l;
  }
  // camino devuelve, para cada mapa, el recorrido de un origen a un agente y dónde están la entrada, el centro y la
  // salida de Tramtai (en % del camino).
  function camino(origen, agente) {
    var yLinea = parseFloat(LINEAS[agente].split(' ')[2]);
    var dAncho = ORIGENES[origen] + ' L 460 ' + yLinea + ' ' + LINEAS[agente].replace(/^M/, 'L');
    var total = largo(ancho, dAncho);
    var entrada = largo(ancho, ORIGENES[origen]);
    var salida = entrada + Math.hypot(170, yLinea - parseFloat(ORIGENES[origen].split(' ').pop()));
    var oy = ALTO_ORIGEN[origen], ly = ALTO_LINEA[agente];
    var dAlto = 'M 18 ' + oy + ' L 18 ' + ly + ' L 52 ' + ly;
    var totalAlto = (ly - oy) + 34;
    return [
      { d: dAncho, entrada: entrada / total * 100, centro: (entrada + salida) / 2 / total * 100, salida: salida / total * 100 },
      { d: dAlto, entrada: (120 - oy) / totalAlto * 100, centro: (149 - oy) / totalAlto * 100, salida: (178 - oy) / totalAlto * 100 }
    ];
  }

  var actual = 0, pausado = quieto, temporizador = null, animaciones = [], iniciado = false;
  var leyenda = caja.querySelector('.mapa-leyenda');
  var puntos = caja.querySelectorAll('.mapa-punto');
  var pausa = caja.querySelector('.mapa-pausa');

  function parar() {
    animaciones.forEach(function (a) { a.cancel(); });
    animaciones = [];
    clearTimeout(temporizador);
  }
  function mover(i, desde, hasta, ms, retraso) {
    var a = trams[i].animate([{ offsetDistance: desde + '%' }, { offsetDistance: hasta + '%' }],
      { duration: ms, delay: retraso || 0, fill: 'forwards', easing: 'ease-in-out' });
    animaciones.push(a);
    return a;
  }
  function mostrar(n) {
    var e = ejemplos[n];
    leyenda.innerHTML = '';
    [['es', ' de '], ['en', ' of ']].forEach(function (par) {
      var s = document.createElement('span');
      s.lang = par[0];
      s.textContent = (n + 1) + par[1] + ejemplos.length + ' · ' + e[par[0]];
      leyenda.appendChild(s);
    });
    puntos.forEach(function (p, i) { p.setAttribute('aria-pressed', String(i === n)); });
    ejemplos.forEach(function (x) { var pre = document.getElementById(x.codigo); if (pre) pre.hidden = x !== e; });
  }
  // recorrer anima el ejemplo n en los dos mapas (uno se ve según el ancho) y programa el siguiente.
  function recorrer(n) {
    iniciado = true;
    parar();
    actual = n;
    var e = ejemplos[n];
    mostrar(n);
    var primero = camino(e.origen, e.agentes[0]);
    var segundo = e.agentes[1] !== undefined ? camino(e.origen, e.agentes[1]) : null;
    trams.forEach(function (t, i) {
      t.style.offsetPath = "path('" + primero[i].d + "')";
      t.style.opacity = '1';
      t.setAttribute('fill', 'var(--acento)');
      if (quieto || pausado) {
        var fin = segundo || primero;
        t.style.offsetPath = "path('" + fin[i].d + "')";
        t.style.offsetDistance = '100%';
        return;
      }
      if (!segundo) {
        mover(i, 0, primero[i].centro, 1800);
        mover(i, primero[i].centro, 100, 2400, 1800 + (e.pausaCentro ? 900 : 200));
        return;
      }
      var mitad = primero[i].salida + (100 - primero[i].salida) / 2;
      mover(i, 0, mitad, 2200).finished.then(function () {
        t.setAttribute('fill', ROJO);
        return mover(i, mitad, primero[i].centro, 900, 400).finished;
      }).then(function () {
        t.setAttribute('fill', 'var(--acento)');
        t.style.offsetPath = "path('" + segundo[i].d + "')";
        return mover(i, segundo[i].centro, 100, 1800, 200).finished;
      }).catch(function () {});
    });
    if (!quieto && !pausado) temporizador = setTimeout(function () { anunciar(false); recorrer((actual + 1) % ejemplos.length); }, 6500);
  }

  // La leyenda se anuncia cuando la elige la persona, no cada vez que rota sola.
  function anunciar(si) { leyenda.setAttribute('aria-live', si ? 'polite' : 'off'); }
  puntos.forEach(function (p, i) { p.addEventListener('click', function () { anunciar(true); recorrer(i); }); });
  pausa.addEventListener('click', function () {
    pausado = !pausado;
    anunciar(true);
    pausa.innerHTML = pausado ? '<span lang="es">Seguir</span><span lang="en">Play</span>' : '<span lang="es">Pausa</span><span lang="en">Pause</span>';
    recorrer(actual);
  });
  // En una pestaña oculta no se anima ni se acumulan animaciones; al volver sigue donde iba.
  document.addEventListener('visibilitychange', function () {
    if (document.hidden) parar();
    else if (iniciado) recorrer(actual);
  });
  // «Reducir movimiento» se respeta también si cambia con la página abierta.
  movimiento.addEventListener('change', function () {
    quieto = movimiento.matches;
    pausado = quieto;
    pausa.hidden = quieto;
    pausa.innerHTML = '<span lang="es">Pausa</span><span lang="en">Pause</span>';
    if (iniciado) recorrer(actual);
  });
  if (quieto) pausa.hidden = true;
  caja.hidden = false;
  mostrar(0);
  // Medir los caminos y animar el tranvía espera a que el mapa esté cerca de la pantalla: no cuesta nada al cargar.
  if ('IntersectionObserver' in window) {
    var vigia = new IntersectionObserver(function (entradas) {
      if (!entradas[entradas.length - 1].isIntersecting) return;
      vigia.disconnect();
      if (!iniciado) recorrer(actual);
    }, { rootMargin: '200px 0px' });
    vigia.observe(caja);
  } else recorrer(0);
})();

// Demo del recorrido guiado: una pantalla de la app por paso de la bienvenida, un paso cada 6 s, con el mismo patrón que
// el recorrido del tranvía: pausa que cambia su texto, un punto con nombre por paso, la leyenda solo se anuncia cuando
// elige la persona, quieta con «reducir movimiento» y sin avanzar con la pestaña oculta ni fuera de la pantalla. Sin
// JavaScript se ven la primera imagen y la lista de pasos.
(function () {
  var caja = document.querySelector('.demo');
  var controles = caja && caja.querySelector('.demo-controles');
  if (!controles) return;
  var figuras = caja.querySelectorAll('.demo-paso');
  var lista = caja.querySelector('.demo-lista');
  var puntos = caja.querySelectorAll('.demo-punto');
  var leyenda = caja.querySelector('.demo-leyenda');
  var pausa = caja.querySelector('.demo-pausa');
  var movimiento = window.matchMedia('(prefers-reduced-motion: reduce)');
  var CADA_MS = 6000;
  var actual = 0, pausado = movimiento.matches, enPantalla = !('IntersectionObserver' in window), temporizador = null;

  function mostrar(n) {
    actual = n;
    figuras.forEach(function (f, i) { f.hidden = i !== n; });
    puntos.forEach(function (p, i) { p.setAttribute('aria-pressed', String(i === n)); });
    leyenda.innerHTML = '';
    [['es', ' de '], ['en', ' of ']].forEach(function (par) {
      var s = document.createElement('span');
      s.lang = par[0];
      s.textContent = (n + 1) + par[1] + figuras.length + ' · ' + lista.children[n].querySelector('[lang="' + par[0] + '"]').textContent;
      leyenda.appendChild(s);
    });
  }
  // La leyenda se anuncia cuando la elige la persona, no cada vez que avanza sola.
  function anunciar(si) { leyenda.setAttribute('aria-live', si ? 'polite' : 'off'); }
  function programar() {
    clearTimeout(temporizador);
    if (pausado || !enPantalla || document.hidden) return;
    temporizador = setTimeout(function () {
      anunciar(false);
      mostrar((actual + 1) % figuras.length);
      programar();
    }, CADA_MS);
  }
  function rotulo() {
    pausa.innerHTML = pausado ? '<span lang="es">Seguir</span><span lang="en">Play</span>' : '<span lang="es">Pausa</span><span lang="en">Pause</span>';
  }

  puntos.forEach(function (p, i) { p.addEventListener('click', function () { anunciar(true); mostrar(i); programar(); }); });
  pausa.addEventListener('click', function () {
    pausado = !pausado;
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
    pausado = movimiento.matches;
    pausa.hidden = pausado;
    rotulo();
    programar();
  });
  pausa.hidden = pausado;
  rotulo();
  lista.hidden = true;
  controles.hidden = false;
  mostrar(0);
  programar();
})();
