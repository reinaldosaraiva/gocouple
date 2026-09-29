(function () {
  var slider = document.getElementById("frame");
  var trail = document.getElementById("trail");

  function setHidden(el, hidden) {
    if (hidden) { el.setAttribute("hidden", ""); } else { el.removeAttribute("hidden"); }
  }

  function showFrame(i) {
    document.querySelectorAll("[data-frame]").forEach(function (el) {
      setHidden(el, el.getAttribute("data-frame") !== String(i));
    });
  }
  if (slider) {
    slider.addEventListener("input", function () { showFrame(slider.value); });
    showFrame(slider.value);
  }
  if (trail) {
    trail.addEventListener("change", function () {
      document.querySelectorAll("g.trail-group").forEach(function (g) {
        setHidden(g, g.getAttribute("data-pkg") !== trail.value);
      });
    });
  }

  document.querySelectorAll("table.sortable th button").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var th = btn.parentNode;
      var table = th.closest("table");
      var idx = Array.prototype.indexOf.call(th.parentNode.children, th);
      var numeric = th.getAttribute("data-num") === "1";
      var dir = th.getAttribute("data-dir") === "asc" ? "desc" : "asc";
      th.parentNode.querySelectorAll("th").forEach(function (h) {
        h.removeAttribute("data-dir");
        h.removeAttribute("aria-sort");
      });
      th.setAttribute("data-dir", dir);
      th.setAttribute("aria-sort", dir === "asc" ? "ascending" : "descending");
      var body = table.tBodies[0];
      var rows = Array.prototype.slice.call(body.rows);
      rows.sort(function (a, b) {
        var x = a.cells[idx].getAttribute("data-sort");
        var y = b.cells[idx].getAttribute("data-sort");
        var c;
        if (numeric) { c = (parseFloat(x) || 0) - (parseFloat(y) || 0); }
        else { c = x < y ? -1 : (x > y ? 1 : 0); }
        return dir === "asc" ? c : -c;
      });
      rows.forEach(function (r) { body.appendChild(r); });
    });
  });

  document.querySelectorAll("svg.dep-graph").forEach(function (svg) {
    function clear() {
      svg.classList.remove("dim");
      svg.querySelectorAll(".hl").forEach(function (el) { el.classList.remove("hl"); });
    }
    svg.querySelectorAll("g.node").forEach(function (node) {
      function on() {
        clear();
        svg.classList.add("dim");
        var id = node.getAttribute("data-id");
        var near = (node.getAttribute("data-nb") || "").split(" ");
        node.classList.add("hl");
        svg.querySelectorAll("g.node").forEach(function (o) {
          if (near.indexOf(o.getAttribute("data-id")) >= 0) { o.classList.add("hl"); }
        });
        svg.querySelectorAll(".edge").forEach(function (e) {
          if (e.getAttribute("data-a") === id || e.getAttribute("data-b") === id) { e.classList.add("hl"); }
        });
      }
      node.addEventListener("mouseenter", on);
      node.addEventListener("focus", on);
      node.addEventListener("mouseleave", clear);
      node.addEventListener("blur", clear);
    });
  });
})();
