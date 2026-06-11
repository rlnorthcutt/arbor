/*!
 * Dark Mode Toggle Web Element
 * Version: 1.1.2
 * URL: https://github.com/rlnorthcutt/dark-mode-toggle
 * License: MIT (https://opensource.org/licenses/MIT)
 */
(() => {
  "use strict";

  /* ================================================================================================
   * Template & CSS (same structure as before)
   * ============================================================================================== */
  const TPL = document.createElement("template");
  TPL.innerHTML = `
    <button part="button" class="track" type="button" role="switch" aria-checked="false" aria-label="Toggle dark mode">
      <span class="icon sun" part="sun" aria-hidden="true"><slot name="sun">
    <svg viewBox="0 0 24 24" aria-hidden="true">
      <path d="M18.1 5.1c0 .3-.1.6-.3.9l-1.4 1.4-.9-.8 2.2-2.2c.3.1.4.4.4.7zm-.5 5.3h3.2c0 .3-.1.6-.4.9s-.5.4-.8.4h-2v-1.3zm-6.2-5V2.2c.3 0 .6.1.9.4s.4.5.4.8v2h-1.3zm6.4 11.7c-.3 0-.6-.1-.8-.3l-1.4-1.4.8-.8 2.2 2.2c-.2.2-.5.3-.8.3zM6.2 4.9c.3 0 .6.1 .8 .3l1.4 1.4-.8 .9-2.2-2.3c.2-.2 .5-.3 .8-.3zm5.2 11.7h1.2v3.2c-.3 0 -.6-.1 -.9-.4s-.4-.5 -.4-.8l.1-2zm-7-6.2h2v1.2H3.2c0-.3 .1-.6 .4-.9s .5-.3 .8-.3z"/>
      <circle cx="12" cy="11" r="4"/>
    </svg>
  </slot></span>
      <span class="icon moon" part="moon" aria-hidden="true"><slot name="moon">
    <svg viewBox="0 0 20 20" aria-hidden="true">
      <path d="M17.39 15.14A7.33 7.33 0 0 1 11.75 1.6c.23-.11 .56-.23 .79-.34a8.19 8.19 0 0 0-5.41 .45 9 9 0 1 0 7 16.58 8.42 8.42 0 0 0 4.29-3.84 5.3 5.3 0 0 1-1.03 .69z"/>
    </svg>
  </slot></span>
      <span class="thumb" part="thumb" aria-hidden="true"></span>
    </button>
  `;

  const CSS = `
    :host{display:inline-block;vertical-align:middle;color-scheme:light dark}
    .track{cursor:pointer;display:flex;align-items:center;justify-content:space-between;padding:0 4px;
      width:var(--track-width,60px);height:var(--track-height,30px);background:var(--track-bg-light,#E9E9EA);
      border-radius:30px;position:relative;border:0;outline:none;transition:background-color .25s ease}
    .track:focus-visible{outline:2px solid color-mix(in oklab,CanvasText 30%,transparent);outline-offset:2px}

    /* Center the thumb vertically regardless of height */
    .thumb{
      position:absolute;top:50%;left:2px;
      width:calc(var(--track-height,30px) - 4px);
      height:calc(var(--track-height,30px) - 4px);
      background:var(--thumb-bg,#fff);border-radius:50%;box-shadow:0 1px 3px rgba(0,0,0,.2);
      transform:translateY(-50%);
      transition:transform .25s cubic-bezier(.25,.46,.45,.94),background-color .25s ease
    }

    /* Icons */
    .icon{display:inline-flex;align-items:center;justify-content:center;
      width:calc(var(--track-height,30px) - 8px);height:calc(var(--track-height,30px) - 8px);
      pointer-events:none;z-index:1;line-height:0}
    .icon svg{display:block;width:100%;height:100%;fill:currentColor;transition:color .25s ease,fill .25s ease}

    .sun{color:var(--icon-sun-color,#000)}
    .moon{color:var(--icon-moon-color,#8E8E93)}

    :host([_state="dark"]) .track{background:var(--track-bg-dark,#4D4D52)}
    :host([_state="dark"]) .thumb{transform:translate(calc(var(--track-width,60px) - var(--track-height,30px)),-50%);background:var(--thumb-bg-dark,#888)}
    :host([_state="dark"]) .sun{color:var(--icon-sun-color-inactive,#8E8E93)}
    :host([_state="dark"]) .moon{color:var(--icon-moon-color-active,#fff)}

    @media (prefers-reduced-motion:reduce){.thumb,.track,.icon svg{transition:none}}
  `;

  let sheet = null;
  try { sheet = new CSSStyleSheet(); sheet.replaceSync(CSS); } catch {}

  /* ================================================================================================
   * Constants & helpers
   * ============================================================================================== */
  const KEY = "dm:theme";
  const CH_NAME = KEY;
  const MQ = matchMedia("(prefers-color-scheme: dark)");
  const isDarkOS = () => MQ.matches;
  const getRoot = (sel) => (sel && sel !== "html" ? (document.querySelector(sel) || document.documentElement) : document.documentElement);

  let bc;
  try { bc = new BroadcastChannel(CH_NAME); } catch {}

  /* ================================================================================================
   * Element definition
   * ============================================================================================== */
  class DarkModeToggle extends HTMLElement {
    static get observedAttributes() {
      // NOTE: added 'light-class' for mutual exclusivity control
      return ["theme","strategy","root","dark-class","light-class","persist","disabled","label"];
    }

    constructor() {
      super();
      const root = this.attachShadow({ mode: "open", delegatesFocus: true });
      if (sheet && root.adoptedStyleSheets) root.adoptedStyleSheets = [sheet];
      else { const s = document.createElement("style"); s.textContent = CSS; root.appendChild(s); }
      root.appendChild(TPL.content.cloneNode(true));

      this._btn = root.querySelector("button");
      this._mo = new MutationObserver(() => this._reflectFromRoot());
      this._onClick = this._onClick.bind(this);
      this._onKey = this._onKey.bind(this);
      this._onStorage = this._onStorage.bind(this);
      this._onMQ = () => this._apply();
      this._onBC = (e) => { if (e?.data?.type === "themechange") this._applyFromExternal(e.data.mode); };
    }

    /* -------------------------------- Lifecycle -------------------------------- */
    connectedCallback() {
      if (!this.hasAttribute("theme")) this.setAttribute("theme","auto");
      if (!this.hasAttribute("strategy")) this.setAttribute("strategy","attr");
      if (!this.hasAttribute("dark-class")) this.setAttribute("dark-class","dark");
      // light-class defaults via getter; no need to set attribute
      this._btn.setAttribute("aria-label", this.label);
      this._syncDisabled();

      this._btn.addEventListener("click", this._onClick);
      this._btn.addEventListener("keydown", this._onKey);
      window.addEventListener("storage", this._onStorage);
      MQ.addEventListener?.("change", this._onMQ);
      bc?.addEventListener?.("message", this._onBC);

      this._initFromRootOrStorage();
      this._apply();
      this._observeRoot(true);
    }

    disconnectedCallback() {
      this._btn.removeEventListener("click", this._onClick);
      this._btn.removeEventListener("keydown", this._onKey);
      window.removeEventListener("storage", this._onStorage);
      MQ.removeEventListener?.("change", this._onMQ);
      bc?.removeEventListener?.("message", this._onBC);
      this._observeRoot(false);
    }

    attributeChangedCallback(name, oldV, newV) {
      if (oldV === newV) return;

      if (name === "strategy" || name === "root") {
        this._observeRoot(false);
        const root = getRoot(this.rootSelector);
        const isClass = (this.strategy || "attr").toLowerCase() === "class";
        if (isClass) {
          // switching to class strategy -> remove data-theme
          root.removeAttribute("data-theme");
        } else {
          // switching to attr strategy -> remove both theme classes
          root.classList.remove(this.darkClass, this.lightClass);
        }
        this._observeRoot(true);
      }

      if (name === "label") this._btn.setAttribute("aria-label", this.label);
      if (name === "disabled") this._syncDisabled();

      this._apply();
    }

    /* ---------------------------- Public API (props) --------------------------- */
    get theme() { return (this.getAttribute("theme") || "auto").toLowerCase(); }
    set theme(v) { this.setAttribute("theme", v); }

    get strategy() { return (this.getAttribute("strategy") || "attr").toLowerCase(); }
    get rootSelector() { return this.getAttribute("root") || "html"; }
    get darkClass() { return this.getAttribute("dark-class") || "dark"; }
    get lightClass() { return this.getAttribute("light-class") || "light"; }
    get persist() { return this.getAttribute("persist") !== "off"; }
    get label() { return this.getAttribute("label") || "Toggle dark mode"; }

    /* -------------------------------- Handlers -------------------------------- */
    _onClick() {
      if (this.hasAttribute("disabled")) return;
      const next = this._current() === "dark" ? "light" : "dark";
      this.theme = next;
      if (this.persist) { try { localStorage.setItem(KEY, next); } catch {} }
      this._broadcast(next);
      this._apply();
    }
    _onKey(e) {
      if (this.hasAttribute("disabled")) return;
      if (e.key === " " || e.key === "Enter") { e.preventDefault(); this._onClick(); }
    }
    _onStorage(e) {
      if (e.key !== KEY) return;
      const v = e.newValue;
      this.theme = (v === "light" || v === "dark") ? v : "auto";
    }

    /* ---------------------------- Core state logic ---------------------------- */
    _current() {
      const attr = this.theme;
      if (attr === "light" || attr === "dark") return attr;

      try {
        const stored = localStorage.getItem(KEY);
        if (stored === "light" || stored === "dark") return stored;
      } catch {}

      const root = getRoot(this.rootSelector);
      if (this.strategy === "class") {
        if (root.classList.contains(this.darkClass)) return "dark";
        if (root.classList.contains(this.lightClass)) return "light";
      } else {
        const t = root.getAttribute("data-theme");
        if (t === "dark") return "dark";
        if (t === "light") return "light";
      }

      return isDarkOS() ? "dark" : "light";
    }

    _apply() {
      const mode = this._current();

      // component state + a11y
      this.toggleAttribute("_state", false);
      this.setAttribute("_state", mode);
      this._btn.setAttribute("aria-checked", String(mode === "dark"));

      const root = getRoot(this.rootSelector);
      if (this.strategy === "class") {
        // ENSURE MUTUAL EXCLUSIVITY
        root.classList.remove(this.darkClass, this.lightClass);
        root.classList.add(mode === "dark" ? this.darkClass : this.lightClass);
        if (root === document.documentElement) document.documentElement.removeAttribute("data-theme");
      } else {
        root.setAttribute("data-theme", mode === "dark" ? "dark" : "light");
        root.classList.remove(this.darkClass, this.lightClass);
      }

      // UA hint
      try { root.style.colorScheme = mode; } catch {}
      if (root !== document.documentElement) {
        try { document.documentElement.style.colorScheme = mode; } catch {}
      }

      this.dispatchEvent(new CustomEvent("change", { detail: { mode } }));
    }

    /* ------------------------------ Cross-context ----------------------------- */
    _broadcast(mode) {
      try { bc?.postMessage({ type: "themechange", mode }); } catch {}
      window.dispatchEvent(new CustomEvent("themechange", { detail: { mode } }));
    }
    _applyFromExternal(mode) {
      if (mode === "light" || mode === "dark") {
        this.theme = mode;
        if (this.persist) { try { localStorage.setItem(KEY, mode); } catch {} }
      }
      this._apply();
    }

    /* -------------------------------- Bootstrap -------------------------------- */
    _initFromRootOrStorage() {
      const root = getRoot(this.rootSelector);
      let found = null;
      if (this.strategy === "class") {
        if (root.classList.contains(this.darkClass)) found = "dark";
        else if (root.classList.contains(this.lightClass)) found = "light";
        else found = null;
      } else {
        const t = root.getAttribute("data-theme");
        found = (t === "dark" || t === "light") ? t : null;
      }
      if (found && this.persist) { try { localStorage.setItem(KEY, found); } catch {} }
    }

    _observeRoot(enable) {
      const root = getRoot(this.rootSelector);
      if (!root) return;
      if (enable) {
        this._mo.observe(root, { attributes: true, attributeFilter: this.strategy === "class" ? ["class"] : ["data-theme"] });
      } else {
        this._mo.disconnect();
      }
    }

    _reflectFromRoot() {
      if (this.theme !== "auto") return;
      const root = getRoot(this.rootSelector);
      const externalDark = this.strategy === "class"
        ? (root.classList.contains(this.darkClass) || (!root.classList.contains(this.lightClass) && isDarkOS()))
        : root.getAttribute("data-theme") === "dark";

      this.setAttribute("_state", externalDark ? "dark" : "light");
      this._btn.setAttribute("aria-checked", String(externalDark));
      try { root.style.colorScheme = externalDark ? "dark" : "light"; } catch {}
      if (root !== document.documentElement) {
        try { document.documentElement.style.colorScheme = externalDark ? "dark" : "light"; } catch {}
      }
    }

    _syncDisabled() {
      const dis = this.hasAttribute("disabled");
      this._btn.toggleAttribute("disabled", dis);
      this._btn.setAttribute("aria-disabled", String(dis));
    }
  }

  customElements.define("dark-mode-toggle", DarkModeToggle);
})();
