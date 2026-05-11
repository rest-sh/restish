(function () {
  function initRestishPrism() {
    if (typeof Prism === "undefined" || Prism.languages.readable) {
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
      httpdate: {
        alias: "date",
        pattern: /"?(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun), [0-9]{2} [A-Z][a-z]{2} [0-9]{4} [0-9]{2}:[0-9]{2}:[0-9]{2} GMT"?/
      },
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

    const quotedSingle = "'(?:\\\\.|[^\\\\'\\r\\n])*'";
    const quotedDouble = "\"(?:\\\\.|[^\\\\\"\\r\\n])*\"";
    const shorthandOperator = "(?::|==|\\bcontains\\b|\\bstartsWith\\b)";
    const shorthandField = "[a-z_][a-z0-9_.-]*(?:\\[[^\\]\\r\\n]*\\])?\\s*" + shorthandOperator;
    const singleQuotedShorthand = "'(?:(?:" + shorthandField + ")|(?:\\\\.|[^\\\\'\\r\\n])*?[\\s,{\\[.]" + shorthandField + ")(?:\\\\.|[^\\\\'\\r\\n])*'";
    const doubleQuotedShorthand = "\"(?:(?:" + shorthandField + ")|(?:\\\\.|[^\\\\\"\\r\\n])*?[\\s,{\\[.]" + shorthandField + ")(?:\\\\.|[^\\\\\"\\r\\n])*\"";
    const quotedFilter = "(?:--rsh-filter|-f)\\s+(?:" + quotedSingle + "|" + quotedDouble + ")";
    const bareFilter = "(?:--rsh-filter|-f)\\s+\\S+";

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
        pattern: /-H\s+(?:[A-Za-z][a-zA-Z0-9-]+:\S+|'[A-Za-z][a-zA-Z0-9-]+:\s*(?:\\.|[^\\'\r\n])*'|"[A-Za-z][a-zA-Z0-9-]+:\s*(?:\\.|[^\\"\r\n])*")/,
        inside: {
          property: /[A-Za-z][a-zA-Z0-9-]+(?=:)/
        }
      },
      queryparam: {
        pattern: /(-[qd] [a-z0-9-]+=\S+|[a-z0-9-]+==\S+)/i,
        inside: {
          property: /[a-z0-9-]+(?==)/i
        }
      },
      shorthand: {
        pattern: new RegExp(quotedFilter + "|" + bareFilter + "|" + singleQuotedShorthand + "|" + doubleQuotedShorthand, "i"),
        greedy: true,
        inside: {
          option: /--rsh-filter|-f/,
          quote: {
            alias: "punctuation",
            pattern: /^['"]|['"]$/
          },
          string: {
            pattern: /"(?:\\.|[^\\"\r\n])*"/,
            greedy: true
          },
          root: {
            alias: "property",
            pattern: /\b(?:body|headers|headers_all|links|status)\b/
          },
          path: {
            alias: "property",
            pattern: /(^|[.{,]\s*)[a-z_][a-z0-9_-]*(?=\s*(?:[.\[{,}]|\]|$))/i,
            lookbehind: true
          },
          property: /[a-z_][a-z0-9_.-]*(?:\[[^\]\r\n]*\])?(?=\s*(?::|==|\bcontains\b|\bstartsWith\b))/i,
          variable: /\$[A-Z0-9_]+/,
          uri: /https?:\/\/[^\s,'"]+|@[^\s,'"]+/,
          number: /\b[0-9]+(\.[0-9]+)?/,
          boolean: /\b(?:true|false)\b/i,
          null: /\b(?:null|undefined)\b/i,
          function: /\b(?:contains|startsWith|select|map|unique|length)\b/,
          operator: /==|:|\||@/,
          punctuation: /[,[\]{}().]/
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
      operator: /<|>|\|/,
      keyword: /\b(for|do|done)(?!\/)\b/
    };

    if (!Prism.languages.json) {
      Prism.languages.json = {
        property: {
          pattern: /"(?:\\.|[^\\"\r\n])*"(?=\s*:)/,
          greedy: true
        },
        string: {
          pattern: /"(?:\\.|[^\\"\r\n])*"/,
          greedy: true
        },
        number: /-?\b\d+(?:\.\d+)?(?:e[+-]?\d+)?\b/i,
        boolean: /\b(?:true|false)\b/,
        null: /\bnull\b/,
        operator: /:/,
        punctuation: /[{}[\],]/
      };
    }

    Prism.languages.sh = Prism.languages.bash;
    Prism.languages.shell = Prism.languages.bash;
    Prism.languages.jsonc = Prism.languages.json;
  }

  if (typeof Prism === "undefined") {
    document.addEventListener("DOMContentLoaded", initRestishPrism, { once: true });
    window.addEventListener("load", initRestishPrism, { once: true });
    return;
  }

  initRestishPrism();
})();
