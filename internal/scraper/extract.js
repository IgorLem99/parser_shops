// Универсальное извлечение товаров. CFG подставляется из Go (селекторы магазина).
(CFG => {
  const clean = el => {
    if (!el) return "";
    const c = el.cloneNode(true);
    if (CFG.remove) c.querySelectorAll(CFG.remove).forEach(s => s.remove());
    return c.textContent.replace(/\s+/g, " ").trim();
  };
  // "селектор@атрибут" -> [селектор, атрибут]
  const split = spec => {
    const m = /^(.*?)@([\w:-]+)$/.exec(spec || "");
    return m ? [m[1].trim(), m[2]] : [(spec || "").trim(), ""];
  };
  const attrOf = (el, attr) => {
    // href/src через свойство — так получаем абсолютный URL
    if ((attr === "href" || attr === "src") && typeof el[attr] === "string" && el[attr]) return el[attr];
    return el.getAttribute(attr) || "";
  };
  const val = (root, spec) => {
    if (!spec) return "";
    const [sel, attr] = split(spec);
    const el = sel ? root.querySelector(sel) : root;
    if (!el) return "";
    if (attr) {
      const v = attrOf(el, attr).replace(/\s+/g, " ").trim();
      if (v) return v;
    }
    return clean(el);
  };
  const link = (root, spec) => {
    const [sel, attr] = split(spec);
    const el = sel ? root.querySelector(sel) : root;
    if (!el) return "";
    return attr ? attrOf(el, attr) : (el.href || el.getAttribute("href") || "");
  };

  let category = "";
  if (CFG.category.fixed) category = CFG.category.fixed;
  else if (CFG.category.selector) category = val(document, CFG.category.selector);
  else if (CFG.category.title_split && document.title.includes(CFG.category.title_split))
    category = document.title.split(CFG.category.title_split)[0].trim();
  if (!category) category = document.title.trim();

  const products = Array.from(document.querySelectorAll(CFG.card)).map(el => {
    const extra = {};
    for (const [k, spec] of Object.entries(CFG.extra || {})) extra[k] = val(el, spec);
    return {
      category,
      name: val(el, CFG.name),
      price: val(el, CFG.price),
      old_price: val(el, CFG.old_price),
      unit: val(el, CFG.unit),
      link: link(el, CFG.link),
      image: CFG.image ? link(el, CFG.image) : "",
      extra,
    };
  });

  let next = "";
  if (CFG.next) {
    const n = document.querySelector(CFG.next);
    if (n) next = n.href || "";
  }
  return { products, next };
})
