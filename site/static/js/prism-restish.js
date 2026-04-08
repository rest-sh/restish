(function () {
  if (typeof Prism === "undefined") {
    return;
  }

  Prism.languages.readable = {
    response: {
      alias: "keyword",
      pattern: /^HTTP\/[12]\.[0-9].*/m,
      inside: {
        success: /[23][0-9]{2}\b.*/,
        error: /[45][0-9]{2}\b.*/
      }
    },
    header: {
      pattern: /^[A-Z][a-zA-Z0-9-]+:.*/m,
      inside: {
        property: /[A-Z][a-zA-Z0-9-]+(?=:)/
      }
    },
    property: /^\s+['"]?[a-z0-9-_$]+['"]?(?=:)/im,
    date: /"?20[0-9]{2}-[01][0-9]-[0-9]{2}(T[0-9:+-.]+Z?)?"?/,
    uri: /"([a-z]+:\/\/|\/).*"/,
    string: {
      pattern: /"(?:\\.|[^\\"\r\n])*"(?!\s*:)/,
      greedy: true
    },
    binary: /\b0x[0-9a-f]+.../i,
    number: /\b0x[\dA-Fa-f]+\b|(?:\b\d+\.?\d*|\B\.\d+)(?:[Ee][+-]?\d+)?/,
    boolean: /\b(?:true|false)\b/i,
    null: /\bnull\b/i
  };

  Prism.languages.bash = {
    comment: /^\s*#.*/m,
    redirect: /2>\/dev\/null/,
    response: {
      pattern: /^(HTTP\/[12].*$)|((\{|\[)(.|\n)+(\]|\}(\n|$)))/gm,
      greedy: false,
      inside: Prism.languages.readable
    },
    uri: {
      pattern: /['"]?(https?:\/\/)?[a-z0-9.-]+\.(com|org|dev|sh)[\S{}]*['"]?/
    },
    shorturi: {
      alias: "uri",
      pattern: /\s[a-z0-9_.-]+\/[a-zA-Z0-9_./?=&{}-]*/
    },
    date: {
      pattern: /"?20[0-9]{2}-[01][0-9]-[0-9]{2}(T[0-9:+-.]+Z?)?"?/
    },
    httpdate: {
      alias: "date",
      pattern: /"?(Mon|Tue|Wed|Thu|Fri|Sat|Sun), [0-9]+ \S+ [0-9]{4} [0-9]{2}:[0-9]{2}:[0-9]{2} GMT"?/
    },
    variable: /[A-Z0-9_]+(?=[=])/,
    varuse: {
      alias: "variable",
      pattern: /\$[A-Z0-9_]+/
    },
    log: {
      pattern: /(INFO|WARN|ERROR): .*/,
      inside: {
        warning: /WARN:/,
        error: /ERROR:/
      }
    },
    header: {
      pattern: /-H [A-Z][a-zA-Z0-9-]+:\S+/,
      inside: {
        property: /[A-Z][a-zA-Z0-9-]+(?=:)/
      }
    },
    queryparam: {
      pattern: /(-[qd] [a-z0-9-]+=\S+|[a-z0-9-]+==\S+)/i,
      inside: {
        property: /[a-z0-9-]+(?==)/i
      }
    },
    string: {
      pattern: /("(?:\\.|[^\\"\r\n])*"(?!\s*:))|('(?:\\.|[^\\'\r\n])*'(?!\s*:))/,
      greedy: true
    },
    keypress: /<\S+>/,
    diffAdded: /\s+added: /,
    diffModified: /\s+modified: /,
    diffRemoved: /\s+removed: /,
    property: /[A-Za-z0-9.-]+(?=[:[{][a-z0-9.^ _\-[\]}]*)/,
    number: /\b[0-9]+(\.[0-9]+)?/,
    boolean: /\b(?:true|false)\b/i,
    null: /\bnull\b/i,
    function: /contains|startsWith/,
    keyword: /restish|rb|<|>|\||\b(for|do|done)(?!\/)\b/
  };

  Prism.languages.sh = Prism.languages.bash;
  Prism.languages.shell = Prism.languages.bash;
  Prism.languages.jsonc = Prism.languages.json;
})();
