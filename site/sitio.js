// Idioma: el elegido antes o, si no, el del navegador. Sin almacenamiento disponible también funciona.
(function () {
  var botones = document.querySelectorAll('[data-idioma]');
  function aplicar(idioma) {
    document.documentElement.lang = idioma;
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
    var texto = pre.textContent.trim();
    var boton = document.createElement('button');
    boton.type = 'button';
    boton.className = 'copiar';
    boton.innerHTML = copiar;
    boton.addEventListener('click', function () {
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
