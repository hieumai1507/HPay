// Theme toggle. The choice is remembered per browser; storage may be blocked, so every access is guarded.
(function () {
  var root = document.documentElement;
  var btn = document.getElementById('theme');
  function stored() { try { return localStorage.getItem('hpay-theme'); } catch (e) { return null; } }
  function save(v) { try { localStorage.setItem('hpay-theme', v); } catch (e) {} }
  var t = stored();
  if (t === 'light' || t === 'dark') root.setAttribute('data-theme', t);
  if (!btn) return;
  btn.addEventListener('click', function () {
    var cur = root.getAttribute('data-theme') ||
      (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    var next = cur === 'dark' ? 'light' : 'dark';
    root.setAttribute('data-theme', next);
    save(next);
  });
})();
