"use strict";
// Copyright (c) 2026 Iqbal Uddin Khan. See LICENSE.
// CInt: a small interpreter for an Arduino C/C++ subset, running on virtual time.
// Supported: setup()/loop(), user functions, int/long/byte/char/bool/float/String, arrays, #define constants,
// if/else/for/while/do/switch, Serial.print/println, delay/millis/micros, pinMode/digitalRead/analogRead/analogWrite/tone/pulseIn,
// DHT objects, map/constrain/min/max/random... Not supported: pointers, structs/classes, interrupts, libraries other than DHT.
(function (root) {
  class CErr extends Error { constructor(msg, line) { super(msg); this.name = "CErr"; this.line = line; } }

  // ------------------------------------------------------------ types
  const TY = {
    int: { k: "i", n: "int16" }, short: { k: "i", n: "int16" }, int16_t: { k: "i", n: "int16" }, long: { k: "i", n: "int32" }, int32_t: { k: "i", n: "int32" },
    "unsigned int": { k: "i", n: "uint16" }, word: { k: "i", n: "uint16" }, uint16_t: { k: "i", n: "uint16" }, "unsigned short": { k: "i", n: "uint16" },
    "unsigned long": { k: "i", n: "uint32" }, uint32_t: { k: "i", n: "uint32" }, size_t: { k: "i", n: "uint32" },
    byte: { k: "i", n: "uint8" }, uint8_t: { k: "i", n: "uint8" }, "unsigned char": { k: "i", n: "uint8" }, char: { k: "c", n: "int8" }, int8_t: { k: "i", n: "int8" }, "signed char": { k: "i", n: "int8" },
    bool: { k: "b", n: "bool" }, boolean: { k: "b", n: "bool" }, float: { k: "f", n: "f32" }, double: { k: "f", n: "f32" },
    String: { k: "s", n: "str" }, void: { k: "v", n: "void" }, DHT: { k: "o", n: "DHT" },
  };
  const TYPEWORDS = new Set(["unsigned", "signed", "long", "short", "int", "char", "float", "double", "void", "bool", "boolean", "byte", "word", "String", "uint8_t", "uint16_t", "uint32_t", "int8_t", "int16_t", "int32_t", "size_t", "DHT", "Servo", "LiquidCrystal"]);
  const QUAL = new Set(["const", "static", "volatile", "extern", "register"]);
  function coerce(n, v) {
    switch (n) {
      case "int16": return (Math.trunc(v) << 16) >> 16;
      case "int32": return Math.trunc(v) | 0;
      case "uint16": return Math.trunc(v) & 0xFFFF;
      case "uint32": return Math.trunc(v) >>> 0;
      case "uint8": return Math.trunc(v) & 0xFF;
      case "int8": return (Math.trunc(v) << 24) >> 24;
      case "bool": return v ? 1 : 0;
      case "f32": return Math.fround(v);
      default: return v;
    }
  }
  const CONSTS = { HIGH: [1, "i"], LOW: [0, "i"], INPUT: [0, "i"], OUTPUT: [1, "i"], INPUT_PULLUP: [2, "i"], LED_BUILTIN: [13, "i"], true: [1, "b"], false: [0, "b"],
    A0: [14, "i"], A1: [15, "i"], A2: [16, "i"], A3: [17, "i"], A4: [18, "i"], A5: [19, "i"], PI: [Math.PI, "f"], TWO_PI: [2 * Math.PI, "f"], HALF_PI: [Math.PI / 2, "f"],
    DEG_TO_RAD: [Math.PI / 180, "f"], RAD_TO_DEG: [180 / Math.PI, "f"], DEC: [10, "i"], HEX: [16, "i"], BIN: [2, "i"], OCT: [8, "i"], DHT11: [11, "i"], DHT22: [22, "i"], DHT21: [21, "i"], NULL: [0, "i"] };
  const ARITY = { pinMode: [2], digitalWrite: [2], digitalRead: [1], analogRead: [1], analogWrite: [2], delay: [1], delayMicroseconds: [1], millis: [0], micros: [0], tone: [2, 3], noTone: [1],
    pulseIn: [2, 3], map: [5], constrain: [3], abs: [1], min: [2], max: [2], pow: [2], sqrt: [1], sq: [1], random: [1, 2], randomSeed: [1], sin: [1], cos: [1], tan: [1], floor: [1], ceil: [1], round: [1], isnan: [1], log: [1], exp: [1] };
  const RTF = new Set(["sqrt", "pow", "sin", "cos", "tan", "asin", "acos", "atan", "atan2", "log", "log10", "exp", "floor", "ceil", "fabs", "radians", "degrees", "fmod"]);
  const POLY = new Set(["abs", "min", "max", "constrain", "sq"]);

  // ------------------------------------------------------------ tokenizer
  const OPS3 = ["<<=", ">>="], OPS2 = ["++", "--", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "==", "!=", "<=", ">=", "&&", "||", "<<", ">>", "->"];
  function tokenize(src, macros, depth, baseLine) {
    const toks = []; let i = 0, line = baseLine || 1; const n = src.length;
    const err = m => { throw new CErr(m, line); };
    while (i < n) {
      const c = src[i];
      if (c === "\n") { line++; i++; continue; }
      if (c === " " || c === "\t" || c === "\r") { i++; continue; }
      if (c === "/" && src[i + 1] === "/") { while (i < n && src[i] !== "\n") i++; continue; }
      if (c === "/" && src[i + 1] === "*") { const e = src.indexOf("*/", i + 2); if (e < 0) err("unterminated comment"); for (let k = i; k < e; k++) if (src[k] === "\n") line++; i = e + 2; continue; }
      if (c === "#" && !depth) {
        let e = src.indexOf("\n", i); if (e < 0) e = n;
        const text = src.slice(i, e).replace(/\/\/.*$/, "").trim();
        let m;
        if ((m = /^#\s*define\s+([A-Za-z_]\w*)\(/.exec(text))) err("function-like macros (#define " + m[1] + "(...)) are not supported; use a function instead");
        if ((m = /^#\s*define\s+([A-Za-z_]\w*)\s+(.+)$/.exec(text))) macros[m[1]] = m[2].trim();
        else if ((m = /^#\s*define\s+([A-Za-z_]\w*)\s*$/.exec(text))) macros[m[1]] = "1";
        i = e; continue;
      }
      if (/[0-9]/.test(c) || (c === "." && /[0-9]/.test(src[i + 1] || ""))) {
        const m = /^(0[xX][0-9a-fA-F]+|0[bB][01]+|(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)([uUlLfF]*)/.exec(src.slice(i, i + 40));
        let v, f = false; const t = m[1];
        if (/^0[xX]/.test(t)) v = parseInt(t, 16); else if (/^0[bB]/.test(t)) v = parseInt(t.slice(2), 2); else { v = parseFloat(t); f = /[.eE]/.test(t) || /[fF]/.test(m[2]); }
        toks.push({ t: "num", v, f, line }); i += m[0].length; continue;
      }
      if (/[A-Za-z_]/.test(c)) {
        const m = /^[A-Za-z_]\w*/.exec(src.slice(i, i + 80)); const id = m[0]; i += id.length;
        if (Object.prototype.hasOwnProperty.call(macros, id)) {
          if (depth > 8) err("macro " + id + " expands recursively");
          const sub = tokenize(macros[id], macros, (depth || 0) + 1, line); sub.forEach(t => toks.push(t)); continue;
        }
        toks.push({ t: "id", v: id, line }); continue;
      }
      if (c === '"' || c === "'") {
        let j = i + 1, s = "";
        while (j < n && src[j] !== c) {
          if (src[j] === "\n") err("missing terminating " + c + " character");
          if (src[j] === "\\") { const e = src[j + 1]; s += ({ n: "\n", t: "\t", r: "\r", "0": "\0", "\\": "\\", '"': '"', "'": "'" })[e] ?? e; j += 2; } else s += src[j++];
        }
        if (j >= n) err("missing terminating " + c + " character");
        i = j + 1;
        if (c === "'") { if (s.length !== 1) err("character constant must hold exactly one character"); toks.push({ t: "chr", v: s.charCodeAt(0), line }); } else toks.push({ t: "str", v: s, line });
        continue;
      }
      let op = null;
      for (const o of OPS3) if (src.startsWith(o, i)) op = o;
      if (!op) for (const o of OPS2) if (src.startsWith(o, i)) op = o;
      if (!op && "+-*/%=<>!&|^~?:;,.(){}[]".includes(c)) op = c;
      if (!op) err("stray '" + c + "' in program");
      toks.push({ t: "op", v: op, line }); i += op.length;
    }
    return toks;
  }

  // ------------------------------------------------------------ parser (AST with static types)
  const PREC = { "||": 1, "&&": 2, "|": 3, "^": 4, "&": 5, "==": 6, "!=": 6, "<": 7, ">": 7, "<=": 7, ">=": 7, "<<": 8, ">>": 8, "+": 9, "-": 9, "*": 10, "/": 10, "%": 10 };
  const ASSIGN = new Set(["=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>="]);

  class Parser {
    constructor(toks) { this.t = toks; this.i = 0; this.funcs = {}; this.sigs = {}; this.globals = []; this.scopes = [new Map()]; }
    peek(o = 0) { return this.t[this.i + o] || { t: "eof", v: "end of file", line: this.t.length ? this.t[this.t.length - 1].line : 1 }; }
    next() { const k = this.peek(); this.i++; return k; }
    isOp(v, o = 0) { const k = this.peek(o); return k.t === "op" && k.v === v; }
    accept(v) { if (this.isOp(v)) { this.i++; return true; } return false; }
    err(m, k) { throw new CErr(m, (k || this.peek()).line); }
    expect(v) {
      if (this.accept(v)) return;
      const k = (v === ";" || v === ")") && this.i > 0 ? this.t[this.i - 1] : this.peek();
      this.err(`expected '${v}' before '${this.peek().v}'`, k);
    }
    ident() { const k = this.next(); if (k.t !== "id") this.err(`expected a name but found '${k.v}'`, k); return k.v; }
    push() { this.scopes.push(new Map()); } pop() { this.scopes.pop(); }
    declare(name, ty, arr, tk) { const s = this.scopes[this.scopes.length - 1]; if (s.has(name)) this.err(`redeclaration of '${name}'`, tk); s.set(name, { ty, arr }); }
    lookup(name) { for (let i = this.scopes.length - 1; i >= 0; i--) if (this.scopes[i].has(name)) return this.scopes[i].get(name); return null; }

    typeAhead(o = 0) { let k = o; while (this.peek(k).t === "id" && QUAL.has(this.peek(k).v)) k++; const t = this.peek(k); return t.t === "id" && TYPEWORDS.has(t.v) ? k : -1; }
    parseType() {
      let any = false;
      while (this.peek().t === "id" && QUAL.has(this.peek().v)) this.i++;
      const words = [];
      while (this.peek().t === "id" && TYPEWORDS.has(this.peek().v)) { words.push(this.next().v); any = true; }
      if (!any) return null;
      if (words.includes("Servo") || words.includes("LiquidCrystal")) this.err(`${words[0]} is not supported by this simulator version`, this.t[this.i - 1]);
      let key;
      const uns = words.includes("unsigned");
      const base = words.filter(w => w !== "unsigned" && w !== "signed");
      if (uns) key = "unsigned " + (base.includes("long") ? "long" : base.includes("short") ? "short" : base.includes("char") ? "char" : "int");
      else if (base.includes("long")) key = "long";
      else if (base.includes("short")) key = "short";
      else key = base[base.length - 1] || "int";
      if (words.includes("signed") && key === "char") key = "signed char";
      const ty = TY[key]; if (!ty) this.err(`unsupported type '${words.join(" ")}'`, this.t[this.i - 1]);
      while (this.isOp("&") || this.isOp("*")) this.err("pointers and references are not supported by this simulator", this.peek());
      return ty;
    }

    prescan() { // collect function signatures so calls may precede definitions
      let depth = 0;
      for (let k = 0; k < this.t.length; k++) {
        const t = this.t[k];
        if (t.t === "op" && t.v === "{") depth++; else if (t.t === "op" && t.v === "}") depth--;
        else if (depth === 0 && t.t === "id" && TYPEWORDS.has(t.v) && !QUAL.has(t.v)) {
          let j = k; while (this.t[j] && this.t[j].t === "id" && (TYPEWORDS.has(this.t[j].v) || QUAL.has(this.t[j].v))) j++;
          if (this.t[j] && this.t[j].t === "id" && this.t[j + 1] && this.t[j + 1].t === "op" && this.t[j + 1].v === "(") {
            const save = this.i; this.i = k; const ty = this.parseType(); this.i = save; if (ty.k !== "o") this.sigs[this.t[j].v] = ty;
          }
          k = j;
        }
      }
    }

    program() {
      this.prescan();
      while (this.peek().t !== "eof") {
        const tk = this.peek(); const ty = this.parseType();
        if (!ty) this.err(`unexpected '${tk.v}' at top level (declarations and functions only)`, tk);
        const nameTok = this.peek(); const name = this.ident();
        if (this.isOp("(") && ty.k !== "o") this.funcDef(ty, name, nameTok); else { this.i--; this.globals.push(this.declRest(ty, true)); }
      }
      if (!this.funcs.setup) throw new CErr("undefined reference to 'setup()' – every sketch needs void setup()", 1);
      if (!this.funcs.loop) throw new CErr("undefined reference to 'loop()' – every sketch needs void loop()", 1);
      return { funcs: this.funcs, globals: this.globals };
    }

    funcDef(ret, name, tk) {
      if (this.funcs[name]) this.err(`redefinition of '${name}'`, tk);
      this.expect("("); const params = [];
      this.push();
      if (this.isOp(")")) { } else if (this.peek().v === "void" && this.isOp(")", 1)) this.i++;
      else do { const pt = this.parseType(); if (!pt) this.err("expected a parameter type", this.peek()); const ptk = this.peek(); const pn = this.ident(); if (this.isOp("[")) this.err("array parameters are not supported", this.peek()); this.declare(pn, pt, false, ptk); params.push({ name: pn, ty: pt }); } while (this.accept(","));
      this.expect(")");
      if (this.isOp(";")) { this.i++; this.pop(); return; } // prototype
      const f = { name, ret, params, body: null, line: tk.line }; this.funcs[name] = f;
      this.cur = f; f.body = this.block(true); this.pop(); this.cur = null;
    }

    block(noScope) { this.expect("{"); if (!noScope) this.push(); const list = []; while (!this.isOp("}")) { if (this.peek().t === "eof") this.err("expected '}' at end of input"); list.push(this.stmt()); } this.i++; if (!noScope) this.pop(); return { k: "block", list }; }

    declRest(ty, global) { // after the type; parses name list and ';'
      const items = [];
      do {
        const tk = this.peek(); const name = this.ident(); const it = { name, ty, init: null, arr: null, ctor: null, line: tk.line };
        if (this.accept("[")) {
          it.arr = { size: this.isOp("]") ? null : this.expr(), init: null }; this.expect("]");
          if (this.isOp("[")) this.err("multi-dimensional arrays are not supported", this.peek());
          if (this.accept("=")) { this.expect("{"); const l = []; if (!this.isOp("}")) do { l.push(this.assign()); } while (this.accept(",") && !this.isOp("}")); this.expect("}"); it.arr.init = l; }
          if (!it.arr.size && !it.arr.init) this.err(`array '${name}' needs a size`, tk);
          this.declare(name, ty, true, tk);
        } else if (ty.k === "o") {
          this.expect("("); it.ctor = []; if (!this.isOp(")")) do { it.ctor.push(this.assign()); } while (this.accept(",")); this.expect(")");
          this.declare(name, ty, false, tk);
        } else {
          if (this.accept("=")) it.init = this.assign();
          this.declare(name, ty, false, tk);
        }
        items.push(it);
      } while (this.accept(","));
      this.expect(";");
      return { k: "decl", items, global: !!global };
    }

    stmt() {
      const tk = this.peek(); const line = tk.line;
      if (this.isOp("{")) return this.block();
      if (this.accept(";")) return { k: "empty" };
      if (tk.t === "id") {
        switch (tk.v) {
          case "if": { this.i++; this.expect("("); const c = this.expr(); this.expect(")"); const a = this.stmt(); let b = null; if (this.peek().v === "else") { this.i++; b = this.stmt(); } return { k: "if", c, a, b, line }; }
          case "while": { this.i++; this.expect("("); const c = this.expr(); this.expect(")"); return { k: "while", c, body: this.stmt(), line }; }
          case "do": { this.i++; const body = this.stmt(); if (this.peek().v !== "while") this.err("expected 'while' after do-body"); this.i++; this.expect("("); const c = this.expr(); this.expect(")"); this.expect(";"); return { k: "do", c, body, line }; }
          case "for": {
            this.i++; this.expect("("); this.push(); let init = null, c = null, inc = null;
            if (!this.isOp(";")) { if (this.typeAhead() >= 0) { const ty = this.parseType(); init = this.declRest(ty); } else { init = { k: "expr", e: this.seq(), line }; this.expect(";"); } } else this.i++;
            if (!this.isOp(";")) c = this.expr(); this.expect(";");
            if (!this.isOp(")")) inc = { k: "expr", e: this.seq(), line }; this.expect(")");
            const body = this.stmt(); this.pop(); return { k: "for", init, c, inc, body, line };
          }
          case "return": { this.i++; let e = null; if (!this.isOp(";")) e = this.expr(); this.expect(";"); return { k: "return", e, line }; }
          case "break": this.i++; this.expect(";"); return { k: "break", line };
          case "continue": this.i++; this.expect(";"); return { k: "continue", line };
          case "switch": {
            this.i++; this.expect("("); const e = this.expr(); this.expect(")"); this.expect("{"); this.push(); const cases = [];
            while (!this.isOp("}")) {
              let test = null;
              if (this.peek().v === "case") { this.i++; test = this.expr(); this.expect(":"); } else if (this.peek().v === "default") { this.i++; this.expect(":"); } else this.err("expected 'case' or 'default'");
              const body = []; while (!this.isOp("}") && this.peek().v !== "case" && this.peek().v !== "default") body.push(this.stmt());
              cases.push({ test, body });
            }
            this.i++; this.pop(); return { k: "switch", e, cases, line };
          }
          case "goto": this.err("goto is not supported by this simulator");
        }
        if (this.typeAhead() >= 0) { const ty = this.parseType(); return { ...this.declRest(ty), line }; }
      }
      const e = this.seq(); this.expect(";"); return { k: "expr", e, line };
    }

    seq() { const l = [this.expr()]; while (this.accept(",")) l.push(this.expr()); return l.length === 1 ? l[0] : { k: "seq", list: l, t: l[l.length - 1].t }; }
    expr() { return this.assign(); }
    assign() {
      const l = this.ternary(); const tk = this.peek();
      if (tk.t === "op" && ASSIGN.has(tk.v)) {
        if (l.k !== "id" && l.k !== "idx") this.err("left side of assignment must be a variable or array element", tk);
        if (l.k === "id" && l.konst !== undefined) this.err(`cannot assign to constant '${l.name}'`, tk);
        this.i++; const r = this.assign(); return { k: "asg", op: tk.v, l, r, t: l.t, line: tk.line };
      }
      return l;
    }
    ternary() {
      const c = this.bin(1);
      if (this.accept("?")) { const a = this.assign(); this.expect(":"); const b = this.assign(); return { k: "tern", c, a, b, t: (a.t === "f" || b.t === "f") ? "f" : a.t }; }
      return c;
    }
    bin(min) {
      let l = this.unary();
      for (; ;) {
        const tk = this.peek(); const p = tk.t === "op" ? PREC[tk.v] : undefined;
        if (!p || p < min) return l;
        this.i++; const r = this.bin(p + 1); const cmp = ["==", "!=", "<", ">", "<=", ">=", "&&", "||"].includes(tk.v);
        let t = "i";
        if (cmp) t = "b"; else if (tk.v === "+" && (l.t === "s" || r.t === "s")) t = "s"; else if (l.t === "f" || r.t === "f") t = "f";
        if ((tk.v === "%" || ["&", "|", "^", "<<", ">>"].includes(tk.v)) && (l.t === "f" || r.t === "f")) this.err(`invalid operands of types 'float' to binary 'operator${tk.v}'`, tk);
        l = { k: "bin", op: tk.v, l, r, t, line: tk.line };
      }
    }
    unary() {
      const tk = this.peek();
      if (tk.t === "op") {
        if (tk.v === "-" || tk.v === "+" || tk.v === "!" || tk.v === "~") { this.i++; const x = this.unary(); return { k: "un", op: tk.v, x, t: tk.v === "!" ? "b" : (x.t === "f" ? "f" : "i") }; }
        if (tk.v === "++" || tk.v === "--") { this.i++; const x = this.unary(); if (x.k !== "id" && x.k !== "idx") this.err("operand of ++/-- must be a variable", tk); return { k: "inc", pre: true, d: tk.v === "++" ? 1 : -1, l: x, t: x.t }; }
        if (tk.v === "(" && this.typeAhead(1) >= 0) { this.i++; const ty = this.parseType(); this.expect(")"); const x = this.unary(); return { k: "cast", ty, x, t: ty.k === "b" ? "i" : ty.k }; }
      }
      if (tk.t === "id" && tk.v === "sizeof") this.err("sizeof is not supported by this simulator");
      return this.postfix();
    }
    postfix() {
      let e = this.primary();
      for (; ;) {
        const tk = this.peek();
        if (tk.t === "op" && (tk.v === "++" || tk.v === "--")) { if (e.k !== "id" && e.k !== "idx") this.err("operand of ++/-- must be a variable", tk); this.i++; e = { k: "inc", pre: false, d: tk.v === "++" ? 1 : -1, l: e, t: e.t }; continue; }
        return e;
      }
    }
    args() { const a = []; this.expect("("); if (!this.isOp(")")) do { a.push(this.assign()); } while (this.accept(",")); this.expect(")"); return a; }
    primary() {
      const tk = this.next();
      switch (tk.t) {
        case "num": return { k: "num", v: tk.v, t: tk.f ? "f" : "i" };
        case "str": return { k: "str", v: tk.v, t: "s" };
        case "chr": return { k: "num", v: tk.v, t: "c" };
        case "op": if (tk.v === "(") { const e = this.seq(); this.expect(")"); return e; } break;
        case "id": {
          const name = tk.v;
          if (name === "F" && this.isOp("(")) { const a = this.args(); return a[0]; }
          if (name === "String" && this.isOp("(")) { const a = this.args(); return { k: "call", name: "String", args: a, t: "s", line: tk.line }; }
          const v = this.lookup(name);
          if (this.isOp(".")) { // method call
            this.i++; const m = this.ident(); const a = this.args(); let t = "i";
            if (name === "Serial") { if (!["begin", "print", "println", "write", "available", "read", "flush", "end", "setTimeout"].includes(m)) this.err(`'Serial.${m}' is not supported by this simulator`, tk); t = m === "available" || m === "read" ? "i" : "v"; }
            else if (v && v.ty.k === "o") { t = ["readTemperature", "readHumidity", "computeHeatIndex", "convertCtoF", "convertFtoC"].includes(m) ? "f" : "v"; if (!["begin", "readTemperature", "readHumidity", "computeHeatIndex", "convertCtoF", "convertFtoC"].includes(m)) this.err(`'${name}.${m}' is not supported by this simulator`, tk); }
            else if (v && v.ty.k === "s") { t = m === "toFloat" ? "f" : m === "charAt" ? "c" : (m === "substring" || m === "trim") ? "s" : "i"; if (!["length", "toInt", "toFloat", "charAt", "substring", "indexOf", "equals", "trim"].includes(m)) this.err(`String method '${m}' is not supported`, tk); }
            else if (!v) this.err(`'${name}' was not declared in this scope`, tk);
            else this.err(`'${name}' has no method '${m}'`, tk);
            return { k: "mcall", obj: name, m, args: a, t, line: tk.line };
          }
          if (this.isOp("(")) {
            const a = this.args(); let t = "i";
            if (this.sigs[name]) t = this.sigs[name].k === "v" ? "v" : (this.sigs[name].k === "b" ? "i" : this.sigs[name].k);
            else if (BUILTINS.has(name)) { if (RTF.has(name)) t = "f"; else if (POLY.has(name)) t = a.some(x => x.t === "f") ? "f" : "i"; if (name === "delay" || name === "delayMicroseconds" || name === "pinMode" || name === "digitalWrite" || name === "analogWrite" || name === "tone" || name === "noTone" || name === "randomSeed") t = "v"; if (name === "isnan") t = "i"; const ar = ARITY[name]; if (ar && !ar.includes(a.length)) this.err(`${a.length < Math.min(...ar) ? "too few" : "too many"} arguments to function '${name}'`, tk); }
            else this.err(`'${name}' was not declared in this scope`, tk);
            if (this.sigs[name]) { const f = this.sigs[name]; }
            return { k: "call", name, args: a, t, line: tk.line };
          }
          if (v) {
            if (v.arr) { if (this.accept("[")) { const i = this.expr(); this.expect("]"); return { k: "idx", name, i, t: v.ty.k === "b" ? "i" : v.ty.k, ty: v.ty, line: tk.line }; } this.err(`array '${name}' must be used with an index`, tk); }
            return { k: "id", name, t: v.ty.k === "b" ? "i" : v.ty.k, ty: v.ty, line: tk.line };
          }
          if (Object.prototype.hasOwnProperty.call(CONSTS, name)) return { k: "id", name, konst: CONSTS[name][0], t: CONSTS[name][1] === "b" ? "i" : CONSTS[name][1] };
          if (name === "Serial") this.err("'Serial' must be used as Serial.print(...) etc.", tk);
          this.err(`'${name}' was not declared in this scope`, tk);
        }
      }
      this.err(`expected an expression before '${tk.v}'`, tk);
    }
  }
  const BUILTINS = new Set(["pinMode", "digitalWrite", "digitalRead", "analogRead", "analogWrite", "delay", "delayMicroseconds", "millis", "micros", "map", "constrain", "min", "max", "abs", "pow", "sqrt", "sq",
    "sin", "cos", "tan", "asin", "acos", "atan", "atan2", "log", "log10", "exp", "floor", "ceil", "round", "fabs", "radians", "degrees", "fmod", "random", "randomSeed", "tone", "noTone", "pulseIn", "isnan", "isinf", "String", "analogReference", "lowByte", "highByte", "bitRead", "bitWrite", "bit"]);

  // ------------------------------------------------------------ machine
  const BRK = { brk: 1 }, CONT = { cont: 1 };
  const SLICE = 1, WAIT = 2;
  const FMT = (v, t, fmt) => {
    if (t === "s") return String(v);
    if (t === "c") return String.fromCharCode(v);
    if (t === "f") { if (!isFinite(v)) return isNaN(v) ? "nan" : (v < 0 ? "-inf" : "inf"); return v.toFixed(fmt === undefined ? 2 : fmt); }
    if (fmt === 16 || fmt === 2 || fmt === 8) return (v < 0 ? (v & 0xFFFF) >>> 0 : v).toString(fmt).toUpperCase();
    return String(Math.trunc(v));
  };

  class Machine {
    constructor(src, hw, opts) {
      opts = opts || {}; this.hw = hw; this.out = opts.out || (() => { }); this.warnFn = opts.warn || (() => { });
      this.micros = 0; this.wake = 0; this.limit = 0; this.budget = 0; this.depth = 0; this.baud = 0; this.finished = false; this.error = null; this.gen = null;
      this.cost = opts.cost || 2; this.seed = 1; this.tmp = 0; this.warned = {};
      const macros = {}; const toks = tokenize(src, macros, 0, 1);
      const p = new Parser(toks); const prog = p.program(); this.prog = prog;
      this.G = Object.create(null); this.G.__t = {};
      this.fns = {};
      for (const f of Object.values(prog.funcs)) this.fns[f.name] = { f, run: null };
      for (const f of Object.values(prog.funcs)) this.fns[f.name].run = this.cs(f.body);
      this.globalInit = prog.globals.map(g => this.cs(g));
      if (hw && hw.setClock) hw.setClock(() => this.micros);
    }
    warn(key, msg) { if (!this.warned[key]) { this.warned[key] = 1; this.warnFn(msg); } }
    fail(msg) { throw new Error(msg); }
    tick() { this.micros += this.cost; if (++this.budget > 150000) { this.limit = -1; } return this.micros >= this.limit; }
    rand() { let t = (this.seed += 0x6D2B79F5); t = Math.imul(t ^ (t >>> 15), t | 1); t ^= t + Math.imul(t ^ (t >>> 7), t | 61); return ((t ^ (t >>> 14)) >>> 0) / 4294967296; }

    // --- async-call hoisting: user function calls and delay() inside expressions are evaluated first
    isAsync(n) { return n.k === "call" && (n.name === "delay" || !!this.prog.funcs[n.name]); }
    hoistRoot(e) { const pre = []; const node = this.h(e, pre); return { node, pre }; }
    h(n, pre) {
      if (!n || typeof n !== "object") return n;
      switch (n.k) {
        case "bin": n.l = this.h(n.l, pre); n.r = this.h(n.r, pre); return n;
        case "un": case "cast": n.x = this.h(n.x, pre); return n;
        case "asg": if (n.l.k === "idx") n.l.i = this.h(n.l.i, pre); n.r = this.h(n.r, pre); return n;
        case "inc": if (n.l.k === "idx") n.l.i = this.h(n.l.i, pre); return n;
        case "idx": n.i = this.h(n.i, pre); return n;
        case "tern": n.c = this.h(n.c, pre); n.a = this.h(n.a, pre); n.b = this.h(n.b, pre); return n;
        case "seq": n.list = n.list.map(x => this.h(x, pre)); return n;
        case "mcall": n.args = n.args.map(x => this.h(x, pre)); return n;
        case "call": {
          n.args = n.args.map(x => this.h(x, pre));
          if (this.isAsync(n)) { const id = ++this.tmp; pre.push({ id, call: n }); return { k: "tmp", id, t: n.t }; }
          return n;
        }
      }
      return n;
    }
    expr(e) { // compile an expression with hoisting -> {pre:[gen], f}
      const r = this.hoistRoot(e); const pre = r.pre.map(p => this.compilePre(p)); return { pre, f: this.cx(r.node) };
    }
    compilePre(p) {
      const args = p.call.args.map(a => this.cx(a)); const id = p.id; const vm = this;
      if (p.call.name === "delay") return function* (sc) { const ms = args[0](sc); yield* vm.delay(ms); sc.__t[id] = 0; };
      const fn = this.fns[p.call.name];
      return function* (sc) { const av = args.map(f => f(sc)); sc.__t[id] = yield* vm.callUser(fn.f, fn.run, av); };
    }
    *delay(ms) { ms = Math.trunc(ms); if (ms <= 0) return; this.wake = this.micros + ms * 1000; yield WAIT; }
    *callUser(f, run, args) {
      if (++this.depth > 120) { this.depth = 0; this.fail("stack overflow: recursion is too deep"); }
      const sc = Object.create(this.G); sc.__t = {};
      f.params.forEach((p, i) => { sc[p.name] = { v: coerce(p.ty.n, args[i] === undefined ? 0 : args[i]) }; });
      let sig; try { sig = yield* run(sc); } finally { this.depth--; }
      if (f.ret.k === "v") return 0;
      return coerce(f.ret.n, sig && sig.ret !== undefined ? sig.ret : 0);
    }

    // --- expressions -> closures
    arith(op, a, b, lt, rt) {
      switch (op) {
        case "+": return (lt === "s" || rt === "s") ? this.str(a, lt) + this.str(b, rt) : a + b;
        case "-": return a - b; case "*": return a * b;
        case "/": if (lt !== "f" && rt !== "f") { if (b === 0) this.fail("division by zero"); return Math.trunc(a / b); } return a / b;
        case "%": if (b === 0) this.fail("division by zero"); return a % b;
        case "&": return a & b; case "|": return a | b; case "^": return a ^ b; case "<<": return a << b; case ">>": return a >> b;
      }
      throw new Error("bad operator " + op);
    }
    str(v, t) { return t === "f" ? FMT(v, "f") : t === "c" ? String.fromCharCode(v) : String(v); }
    cx(n) {
      const vm = this;
      switch (n.k) {
        case "num": case "str": { const v = n.v; return () => v; }
        case "id": { if (n.konst !== undefined) { const v = n.konst; return () => v; } const name = n.name; return sc => sc[name].v; }
        case "tmp": { const id = n.id; return sc => sc.__t[id]; }
        case "idx": { const name = n.name, f = this.cx(n.i); return sc => { const a = sc[name].v, i = f(sc) | 0; if (i < 0 || i >= a.length) { vm.warn("oob" + name, `array '${name}' index ${i} is out of range (0..${a.length - 1})`); return 0; } return a[i]; }; }
        case "un": { const f = this.cx(n.x); switch (n.op) { case "-": return sc => -f(sc); case "+": return sc => f(sc); case "!": return sc => f(sc) ? 0 : 1; case "~": return sc => ~f(sc); } break; }
        case "cast": { const f = this.cx(n.x), nm = n.ty.n; if (nm === "str") return sc => String(f(sc)); return sc => coerce(nm, f(sc)); }
        case "tern": { const c = this.cx(n.c), a = this.cx(n.a), b = this.cx(n.b); return sc => c(sc) ? a(sc) : b(sc); }
        case "seq": { const l = n.list.map(x => this.cx(x)); return sc => { let v; for (const f of l) v = f(sc); return v; }; }
        case "bin": {
          const L = this.cx(n.l), R = this.cx(n.r), op = n.op, lt = n.l.t, rt = n.r.t;
          switch (op) {
            case "&&": return sc => (L(sc) && R(sc)) ? 1 : 0; case "||": return sc => (L(sc) || R(sc)) ? 1 : 0;
            case "==": return sc => L(sc) === R(sc) ? 1 : 0; case "!=": return sc => L(sc) !== R(sc) ? 1 : 0;
            case "<": return sc => L(sc) < R(sc) ? 1 : 0; case ">": return sc => L(sc) > R(sc) ? 1 : 0;
            case "<=": return sc => L(sc) <= R(sc) ? 1 : 0; case ">=": return sc => L(sc) >= R(sc) ? 1 : 0;
            case "+": if (n.t === "s") return sc => vm.str(L(sc), lt) + vm.str(R(sc), rt); return sc => L(sc) + R(sc);
            case "-": return sc => L(sc) - R(sc); case "*": return sc => L(sc) * R(sc);
            default: return sc => { const a = L(sc), b = R(sc); return vm.arith(op, a, b, lt, rt); };
          }
        }
        case "asg": {
          const R = this.cx(n.r), op = n.op === "=" ? null : n.op.slice(0, -1), lt = n.l.t, rt = n.r.t;
          if (n.l.k === "id") {
            const name = n.l.name, nm = n.l.ty.n;
            return sc => { const box = sc[name]; const rv = R(sc); const v = op ? vm.arith(op, box.v, rv, lt === "s" ? "s" : lt, rt) : rv; return (box.v = coerce(nm, v)); };
          }
          const name = n.l.name, I = this.cx(n.l.i), nm = n.l.ty.n;
          return sc => { const a = sc[name].v, i = I(sc) | 0, rv = R(sc); if (i < 0 || i >= a.length) { vm.warn("oob" + name, `array '${name}' index ${i} is out of range (0..${a.length - 1})`); return 0; } const v = op ? vm.arith(op, a[i], rv, lt, rt) : rv; return (a[i] = coerce(nm, v)); };
        }
        case "inc": {
          const d = n.d, pre = n.pre;
          if (n.l.k === "id") { const name = n.l.name, nm = n.l.ty.n; return sc => { const b = sc[name]; const old = b.v; b.v = coerce(nm, old + d); return pre ? b.v : old; }; }
          const name = n.l.name, I = this.cx(n.l.i), nm = n.l.ty.n;
          return sc => { const a = sc[name].v, i = I(sc) | 0; if (i < 0 || i >= a.length) return 0; const old = a[i]; a[i] = coerce(nm, old + d); return pre ? a[i] : old; };
        }
        case "mcall": return this.cMethod(n);
        case "call": return this.cCall(n);
      }
      throw new Error("cannot compile " + n.k);
    }
    cMethod(n) {
      const vm = this, args = n.args.map(a => this.cx(a)), ats = n.args.map(a => a.t), obj = n.obj, m = n.m;
      if (obj === "Serial") {
        switch (m) {
          case "begin": return sc => { vm.baud = args[0] ? args[0](sc) : 9600; return 0; };
          case "print": case "println": {
            const nl = m === "println" ? "\n" : "";
            if (!args.length) return () => { vm.out(nl); return 0; };
            return sc => { const v = args[0](sc); const fmt = args[1] ? args[1](sc) : undefined; vm.out(FMT(v, ats[0], fmt) + nl); return 0; };
          }
          case "write": return sc => { vm.out(String.fromCharCode(args[0](sc))); return 0; };
          case "available": return () => 0; case "read": return () => -1; default: return () => 0;
        }
      }
      return sc => {
        const box = sc[obj];
        if (n.t === "s" || typeof box.v === "string") {
          const s = box.v; const a = args.map(f => f(sc));
          switch (m) { case "length": return s.length; case "toInt": return parseInt(s, 10) || 0; case "toFloat": return parseFloat(s) || 0; case "charAt": return s.charCodeAt(a[0]) || 0; case "substring": return s.substring(a[0], a[1]); case "indexOf": return s.indexOf(typeof a[0] === "number" ? String.fromCharCode(a[0]) : a[0]); case "equals": return s === a[0] ? 1 : 0; case "trim": return s.trim(); }
        }
        const o = box.v; const a = args.map(f => f(sc));
        switch (m) {
          case "begin": return 0;
          case "readTemperature": { const r = vm.hw.dht(o.pin); if (!r) return NaN; return a[0] ? r.t * 1.8 + 32 : r.t; }
          case "readHumidity": { const r = vm.hw.dht(o.pin); return r ? r.h : NaN; }
          case "computeHeatIndex": return a[0];
          case "convertCtoF": return a[0] * 1.8 + 32; case "convertFtoC": return (a[0] - 32) / 1.8;
        }
        return 0;
      };
    }
    cCall(n) {
      const vm = this, name = n.name, args = n.args.map(a => this.cx(a)), ats = n.args.map(a => a.t), hw = this.hw;
      if (this.prog.funcs[name] || name === "delay") throw new Error("internal: async call not hoisted: " + name);
      const A = sc => args.map(f => f(sc));
      switch (name) {
        case "pinMode": return sc => { hw.pinMode(args[0](sc), args[1](sc)); return 0; };
        case "digitalWrite": return sc => { hw.digitalWrite(args[0](sc), args[1](sc) ? 1 : 0); return 0; };
        case "digitalRead": return sc => hw.digitalRead(args[0](sc)) ? 1 : 0;
        case "analogRead": return sc => { const p = args[0](sc); return Math.max(0, Math.min(1023, Math.trunc(hw.analogRead(p >= 14 ? p - 14 : p)))); };
        case "analogWrite": return sc => { hw.analogWrite(args[0](sc), Math.max(0, Math.min(255, Math.trunc(args[1](sc))))); return 0; };
        case "delayMicroseconds": return sc => { vm.micros += Math.max(0, args[0](sc)); return 0; };
        case "millis": return () => Math.floor(vm.micros / 1000) >>> 0;
        case "micros": return () => (Math.floor(vm.micros / 4) * 4) >>> 0;
        case "tone": return sc => { hw.tone(args[0](sc), args[1](sc), args[2] ? args[2](sc) : 0); return 0; };
        case "noTone": return sc => { hw.noTone(args[0](sc)); return 0; };
        case "pulseIn": return sc => { const p = args[0](sc), lvl = args[1](sc), to = args[2] ? args[2](sc) : 1000000; const r = hw.pulseIn(p, lvl, to); vm.micros += r.advance; return r.us; };
        case "map": return sc => { const [x, a, b, c, d] = A(sc).map(Math.trunc); if (b === a) vm.fail("map(): in_min equals in_max (division by zero)"); return Math.trunc((x - a) * (d - c) / (b - a)) + c; };
        case "constrain": return sc => { const [x, a, b] = A(sc); return x < a ? a : x > b ? b : x; };
        case "min": return sc => Math.min(args[0](sc), args[1](sc)); case "max": return sc => Math.max(args[0](sc), args[1](sc));
        case "abs": case "fabs": return sc => Math.abs(args[0](sc));
        case "sq": return sc => { const x = args[0](sc); return x * x; };
        case "pow": return sc => Math.pow(args[0](sc), args[1](sc));
        case "sqrt": return sc => Math.sqrt(args[0](sc));
        case "sin": return sc => Math.sin(args[0](sc)); case "cos": return sc => Math.cos(args[0](sc)); case "tan": return sc => Math.tan(args[0](sc));
        case "asin": return sc => Math.asin(args[0](sc)); case "acos": return sc => Math.acos(args[0](sc)); case "atan": return sc => Math.atan(args[0](sc)); case "atan2": return sc => Math.atan2(args[0](sc), args[1](sc));
        case "log": return sc => Math.log(args[0](sc)); case "log10": return sc => Math.log10(args[0](sc)); case "exp": return sc => Math.exp(args[0](sc));
        case "floor": return sc => Math.floor(args[0](sc)); case "ceil": return sc => Math.ceil(args[0](sc));
        case "round": return sc => { const x = args[0](sc); return x < 0 ? -Math.round(-x) : Math.round(x); };
        case "radians": return sc => args[0](sc) * Math.PI / 180; case "degrees": return sc => args[0](sc) * 180 / Math.PI;
        case "fmod": return sc => args[0](sc) % args[1](sc);
        case "isnan": return sc => Number.isNaN(args[0](sc)) ? 1 : 0; case "isinf": return sc => { const x = args[0](sc); return (x === Infinity || x === -Infinity) ? 1 : 0; };
        case "random": return sc => { const a = A(sc); if (a.length === 1) return a[0] <= 0 ? 0 : Math.floor(vm.rand() * a[0]); return a[0] >= a[1] ? a[0] : a[0] + Math.floor(vm.rand() * (a[1] - a[0])); };
        case "randomSeed": return sc => { vm.seed = args[0](sc) | 0; return 0; };
        case "analogReference": return () => 0;
        case "String": return sc => vm.str(args[0](sc), ats[0]);
        case "lowByte": return sc => args[0](sc) & 0xFF; case "highByte": return sc => (args[0](sc) >> 8) & 0xFF;
        case "bitRead": return sc => (args[0](sc) >> args[1](sc)) & 1; case "bit": return sc => 1 << args[0](sc);
        case "bitWrite": return () => 0;
      }
      throw new CErr(`'${name}' is not supported by this simulator`, n.line);
    }

    // --- statements -> generator closures
    cs(n) {
      const vm = this;
      switch (n.k) {
        case "empty": return function* () { };
        case "block": {
          const list = n.list.map(s => this.cs(s)); const scoped = n.list.some(s => s.k === "decl");
          return function* (sc) { const inner = scoped ? Object.create(sc) : sc; for (const s of list) { const sig = yield* s(inner); if (sig) return sig; } };
        }
        case "decl": {
          const items = n.items.map(it => {
            const nm = it.ty.n, name = it.name;
            if (it.arr) { const size = it.arr.size ? this.cx(it.arr.size) : null; const init = it.arr.init ? it.arr.init.map(e => this.cx(e)) : null; return sc => { const len = size ? size(sc) | 0 : init.length; const a = new Array(len).fill(nm === "str" ? "" : 0); if (init) init.forEach((f, i) => { a[i] = coerce(nm, f(sc)); }); sc[name] = { v: a }; }; }
            if (it.ctor) { const as = it.ctor.map(e => this.cx(e)); return sc => { const a = as.map(f => f(sc)); sc[name] = { v: { kind: it.ty.n, pin: a[0], type: a[1] } }; }; }
            const ex = it.init ? this.expr(it.init) : null;
            return function* (sc) { if (ex) { for (const p of ex.pre) yield* p(sc); sc[name] = { v: coerce(nm, ex.f(sc)) }; } else sc[name] = { v: nm === "str" ? "" : 0 }; };
          });
          return function* (sc) { if (vm.tick()) yield SLICE; for (const it of items) { const r = it(sc); if (r && r.next) yield* r; } };
        }
        case "expr": {
          const ex = this.expr(n.e);
          return function* (sc) { if (vm.tick()) yield SLICE; for (const p of ex.pre) yield* p(sc); ex.f(sc); };
        }
        case "if": {
          const c = this.expr(n.c), a = this.cs(n.a), b = n.b ? this.cs(n.b) : null;
          return function* (sc) { if (vm.tick()) yield SLICE; for (const p of c.pre) yield* p(sc); if (c.f(sc)) return yield* a(sc); else if (b) return yield* b(sc); };
        }
        case "while": {
          const c = this.expr(n.c), body = this.cs(n.body);
          return function* (sc) { for (; ;) { if (vm.tick()) yield SLICE; for (const p of c.pre) yield* p(sc); if (!c.f(sc)) break; const sig = yield* body(sc); if (sig === BRK) break; if (sig && sig !== CONT) return sig; } };
        }
        case "do": {
          const c = this.expr(n.c), body = this.cs(n.body);
          return function* (sc) { for (; ;) { if (vm.tick()) yield SLICE; const sig = yield* body(sc); if (sig === BRK) break; if (sig && sig !== CONT) return sig; for (const p of c.pre) yield* p(sc); if (!c.f(sc)) break; } };
        }
        case "for": {
          const init = n.init ? this.cs(n.init.k === "decl" ? n.init : n.init) : null, c = n.c ? this.expr(n.c) : null, inc = n.inc ? this.cs(n.inc) : null, body = this.cs(n.body);
          return function* (sc) {
            const inner = Object.create(sc); if (init) yield* init(inner);
            for (; ;) {
              if (vm.tick()) yield SLICE;
              if (c) { for (const p of c.pre) yield* p(inner); if (!c.f(inner)) break; }
              const sig = yield* body(inner); if (sig === BRK) break; if (sig && sig !== CONT) return sig;
              if (inc) yield* inc(inner);
            }
          };
        }
        case "return": { const ex = n.e ? this.expr(n.e) : null; return function* (sc) { if (vm.tick()) yield SLICE; if (!ex) return { ret: undefined }; for (const p of ex.pre) yield* p(sc); return { ret: ex.f(sc) }; }; }
        case "break": return function* () { return BRK; };
        case "continue": return function* () { return CONT; };
        case "switch": {
          const e = this.expr(n.e); const cases = n.cases.map(c => ({ test: c.test ? this.cx(c.test) : null, body: c.body.map(s => this.cs(s)) }));
          return function* (sc) {
            if (vm.tick()) yield SLICE; for (const p of e.pre) yield* p(sc); const v = e.f(sc);
            let start = cases.findIndex(c => c.test && c.test(sc) === v); if (start < 0) start = cases.findIndex(c => !c.test);
            if (start < 0) return;
            const inner = Object.create(sc);
            for (let i = start; i < cases.length; i++) for (const s of cases[i].body) { const sig = yield* s(inner); if (sig === BRK) return; if (sig) return sig; }
          };
        }
      }
      throw new Error("cannot compile statement " + n.k);
    }

    *main() {
      try {
        for (const g of this.globalInit) yield* g(this.G);
        const s = this.fns.setup; yield* this.callUser(s.f, s.run, []);
        const l = this.fns.loop;
        for (; ;) { yield* this.callUser(l.f, l.run, []); if (this.tick()) yield SLICE; }
      } catch (e) { this.error = e; this.finished = true; throw e; }
    }
    /** Advance the sketch by dtMicros of virtual time. Throws on runtime errors. */
    run(dtMicros) {
      if (this.finished) return;
      if (!this.gen) this.gen = this.main();
      const target = this.micros + dtMicros; this.limit = target; this.budget = 0;
      while (this.micros < target) {
        if (this.wake > this.micros) { const t = Math.min(this.wake, target); this.micros = t; if (this.micros < this.wake) break; this.wake = 0; }
        if (this.limit < 0) { this.limit = target; break; }
        const r = this.gen.next(); if (r.done) { this.finished = true; return; }
        if (r.value === SLICE && this.limit === -1) break;
      }
      this.limit = target;
    }
  }

  const api = { Machine, compile: src => { const macros = {}; return new Parser(tokenize(src, macros, 0, 1)).program(); }, CErr, coerce, FMT };
  if (typeof module !== "undefined" && module.exports) module.exports = api; else root.CInt = api;
})(typeof window !== "undefined" ? window : globalThis);
