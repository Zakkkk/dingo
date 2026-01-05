// Tree-sitter grammar for Dingo language
// Dingo is a superset of Go with additional syntax sugar

const PREC = {
  // Lowest precedence
  composite_literal: -1,

  // Go operators (from lowest to highest)
  or: 1,              // ||
  and: 2,             // &&
  null_coalesce: 3,   // ?? (Dingo)
  compare: 4,         // == != < <= > >=
  add: 5,             // + - | ^
  multiply: 6,        // * / % << >> & &^
  unary: 7,           // ! - + ^ * & <-

  // High precedence
  call: 12,
  member: 13,
  safe_nav: 14,       // ?. (Dingo)
  error_prop: 15,     // ? (Dingo)
};

module.exports = grammar({
  name: 'dingo',

  extras: $ => [
    /\s/,
    $.comment,
  ],

  word: $ => $.identifier,

  conflicts: $ => [
    // Dingo-specific
    [$.rust_style_lambda],
    [$.rust_style_lambda, $.call_expression],
    [$.lambda_parameter, $._expression],
    [$.lambda_parameter, $._expression, $._simple_type],

    // Go grammar ambiguities (comprehensive list from tree-sitter-go)
    [$._simple_type, $._expression],
    [$._simple_type, $.generic_type],
    [$._simple_type, $.generic_type, $._expression],
    [$._simple_type, $.call_expression],
    [$._simple_type, $.interface_type],
    [$._simple_type, $.interface_type, $.method_spec],
    [$._simple_type, $.generic_type, $.interface_type],
    [$.qualified_type, $._expression],
    [$.generic_type, $._expression],
    [$.generic_type, $.call_expression],
    [$.type_parameter, $._expression],
    [$.type_parameter, $.generic_type, $._expression],
    [$.parameter_declaration, $._simple_type],
    [$.call_expression, $.index_expression],
    [$.expression_statement, $.composite_literal],
    [$.keyed_element, $._expression],
    [$.short_var_declaration, $._expression],
    [$.assignment_statement, $._expression],
    [$.range_clause, $._expression],
    [$.range_clause, $.short_var_declaration, $._expression],
    [$.const_spec],
    [$.var_spec],
    [$.function_declaration],
    [$.method_declaration],
    [$.func_literal],
    [$.function_type],
    [$.field_declaration],
    [$.method_spec],
    [$.channel_type],
  ],

  rules: {
    source_file: $ => repeat($._statement),

    // =============================================
    // COMMENTS
    // =============================================
    comment: $ => choice(
      $.line_comment,
      $.block_comment,
    ),

    line_comment: $ => token(seq('//', /.*/)),

    block_comment: $ => token(seq(
      '/*',
      /[^*]*\*+([^/*][^*]*\*+)*/,
      '/'
    )),

    // =============================================
    // STATEMENTS
    // =============================================
    _statement: $ => choice(
      $.package_clause,
      $.import_declaration,
      $.function_declaration,
      $.method_declaration,
      $.type_declaration,
      $.const_declaration,
      $.var_declaration,
      $.short_var_declaration,
      $.assignment_statement,
      $.let_declaration,        // Dingo: let binding
      $.enum_declaration,       // Dingo: enum
      $.expression_statement,
      $.return_statement,
      $.if_statement,
      $.for_statement,
      $.switch_statement,
      $.select_statement,
      $.go_statement,
      $.defer_statement,
      $.block,
      $.empty_statement,
    ),

    empty_statement: $ => ';',

    package_clause: $ => seq('package', $.identifier),

    import_declaration: $ => seq(
      'import',
      choice(
        $.import_spec,
        $.import_spec_list,
      ),
    ),

    import_spec_list: $ => seq('(', repeat(seq($.import_spec, optional(choice(';', '\n')))), ')'),

    import_spec: $ => seq(
      optional(field('alias', choice($.identifier, '.'))),
      field('path', $.interpreted_string_literal),
    ),

    // =============================================
    // DINGO: LET BINDING
    // =============================================
    let_declaration: $ => seq(
      'let',
      field('name', $.identifier),
      optional(seq(':', field('type', $._type))),
      '=',
      field('value', $._expression),
    ),

    // =============================================
    // DINGO: ENUM DECLARATION
    // =============================================
    enum_declaration: $ => seq(
      'enum',
      field('name', $.identifier),
      optional($.type_parameters),
      '{',
      repeat($.enum_variant),
      '}',
    ),

    enum_variant: $ => seq(
      field('name', $.identifier),
      optional($.enum_variant_fields),
      optional(','),
    ),

    enum_variant_fields: $ => choice(
      // Tuple-style: Variant(Type1, Type2)
      seq('(', commaSep($._type), ')'),
      // Struct-style: Variant { field: Type }
      seq('{', commaSep($.enum_field), optional(','), '}'),
    ),

    enum_field: $ => seq(
      field('name', $.identifier),
      ':',
      field('type', $._type),
    ),

    // =============================================
    // DINGO: MATCH EXPRESSION
    // =============================================
    match_expression: $ => seq(
      'match',
      field('subject', $._expression),
      '{',
      repeat($.match_arm),
      '}',
    ),

    match_arm: $ => seq(
      field('pattern', $.pattern),
      optional($.guard_clause),
      '=>',
      field('body', choice($._expression, $.block)),
      optional(','),
    ),

    guard_clause: $ => seq('if', $._expression),

    pattern: $ => choice(
      $.wildcard_pattern,
      $.literal_pattern,
      $.binding_pattern,
      $.variant_pattern,
    ),

    wildcard_pattern: $ => '_',

    binding_pattern: $ => $.identifier,

    variant_pattern: $ => prec(1, seq(
      field('type', $.identifier),
      optional(seq('(', commaSep($.pattern), ')')),
    )),

    literal_pattern: $ => choice(
      $.int_literal,
      $.float_literal,
      $.interpreted_string_literal,
      $.true,
      $.false,
    ),

    // =============================================
    // DINGO: LAMBDA EXPRESSIONS
    // =============================================
    lambda_expression: $ => choice(
      $.rust_style_lambda,
      $.arrow_style_lambda,
    ),

    // Rust-style: |x| expr or |x, y| expr
    rust_style_lambda: $ => prec(PREC.call, seq(
      '|',
      commaSep($.lambda_parameter),
      '|',
      field('body', choice($._expression, $.block)),
    )),

    // TypeScript-style: (x) => expr or x => expr
    arrow_style_lambda: $ => prec.right(PREC.call, seq(
      choice(
        seq('(', commaSep($.lambda_parameter), ')'),
        $.identifier,
      ),
      '=>',
      field('body', choice($._expression, $.block)),
    )),

    lambda_parameter: $ => seq(
      field('name', $.identifier),
      optional(seq(':', field('type', $._type))),
    ),

    // =============================================
    // EXPRESSIONS
    // =============================================
    _expression: $ => choice(
      $.identifier,
      $._literal,
      $.composite_literal,
      $.func_literal,
      $.lambda_expression,         // Dingo
      $.match_expression,          // Dingo
      $.unary_expression,
      $.binary_expression,
      $.error_propagation,         // Dingo: expr?
      $.safe_navigation,           // Dingo: expr?.field
      $.null_coalesce,             // Dingo: expr ?? default
      $.selector_expression,
      $.index_expression,
      $.slice_expression,
      $.call_expression,
      $.type_assertion,
      $.parenthesized_expression,
    ),

    parenthesized_expression: $ => seq('(', $._expression, ')'),

    // Dingo: Error propagation - expr?
    error_propagation: $ => prec.left(PREC.error_prop, seq(
      field('operand', $._expression),
      '?',
    )),

    // Dingo: Safe navigation - a?.b
    safe_navigation: $ => prec.left(PREC.safe_nav, seq(
      field('operand', $._expression),
      '?.',
      field('field', $.identifier),
    )),

    // Dingo: Null coalescing - a ?? b
    null_coalesce: $ => prec.left(PREC.null_coalesce, seq(
      field('left', $._expression),
      '??',
      field('right', $._expression),
    )),

    call_expression: $ => prec(PREC.call, seq(
      field('function', $._expression),
      optional($.type_arguments),
      field('arguments', $.argument_list),
    )),

    argument_list: $ => seq('(', commaSep($._expression), optional(','), ')'),

    selector_expression: $ => prec(PREC.member, seq(
      field('operand', $._expression),
      '.',
      field('field', $.identifier),
    )),

    index_expression: $ => prec(PREC.member, seq(
      field('operand', $._expression),
      '[',
      field('index', $._expression),
      ']',
    )),

    slice_expression: $ => prec(PREC.member, seq(
      field('operand', $._expression),
      '[',
      optional(field('start', $._expression)),
      ':',
      optional(field('end', $._expression)),
      optional(seq(':', field('capacity', $._expression))),
      ']',
    )),

    type_assertion: $ => prec(PREC.member, seq(
      field('operand', $._expression),
      '.',
      '(',
      field('type', $._type),
      ')',
    )),

    unary_expression: $ => prec(PREC.unary, seq(
      field('operator', choice('!', '-', '+', '^', '*', '&', '<-')),
      field('operand', $._expression),
    )),

    binary_expression: $ => {
      const table = [
        [PREC.or, '||'],
        [PREC.and, '&&'],
        [PREC.compare, choice('==', '!=', '<', '<=', '>', '>=')],
        [PREC.add, choice('+', '-', '|', '^')],
        [PREC.multiply, choice('*', '/', '%', '<<', '>>', '&', '&^')],
      ];
      return choice(...table.map(([prec_val, operator]) =>
        prec.left(prec_val, seq(
          field('left', $._expression),
          field('operator', operator),
          field('right', $._expression),
        ))
      ));
    },

    // =============================================
    // LITERALS
    // =============================================
    _literal: $ => choice(
      $.int_literal,
      $.float_literal,
      $.imaginary_literal,
      $.rune_literal,
      $.interpreted_string_literal,
      $.raw_string_literal,
      $.true,
      $.false,
      $.nil,
    ),

    int_literal: $ => token(choice(
      /0[xX][0-9a-fA-F_]+/,
      /0[oO]?[0-7_]+/,
      /0[bB][01_]+/,
      /[0-9][0-9_]*/,
    )),

    float_literal: $ => token(choice(
      /[0-9][0-9_]*\.[0-9_]*([eE][+-]?[0-9_]+)?/,
      /[0-9][0-9_]*[eE][+-]?[0-9_]+/,
      /\.[0-9_]+([eE][+-]?[0-9_]+)?/,
      /0[xX][0-9a-fA-F_]*\.[0-9a-fA-F_]*[pP][+-]?[0-9_]+/,
    )),

    imaginary_literal: $ => token(seq(
      choice(
        /[0-9][0-9_]*/,
        /[0-9][0-9_]*\.[0-9_]*([eE][+-]?[0-9_]+)?/,
        /\.[0-9_]+([eE][+-]?[0-9_]+)?/,
      ),
      'i',
    )),

    rune_literal: $ => token(seq(
      "'",
      choice(
        /[^'\\]/,
        /\\[abfnrtv\\']/,
        /\\x[0-9a-fA-F]{2}/,
        /\\u[0-9a-fA-F]{4}/,
        /\\U[0-9a-fA-F]{8}/,
        /\\[0-7]{3}/,
      ),
      "'",
    )),

    interpreted_string_literal: $ => seq(
      '"',
      repeat(choice(
        token.immediate(prec(1, /[^"\\]+/)),
        $.escape_sequence,
      )),
      '"',
    ),

    raw_string_literal: $ => token(seq('`', /[^`]*/, '`')),

    escape_sequence: $ => token.immediate(seq(
      '\\',
      choice(
        /[abfnrtv\\'\"]/,
        /x[0-9a-fA-F]{2}/,
        /u[0-9a-fA-F]{4}/,
        /U[0-9a-fA-F]{8}/,
        /[0-7]{3}/,
      ),
    )),

    true: $ => 'true',
    false: $ => 'false',
    nil: $ => 'nil',

    // =============================================
    // COMPOSITE LITERALS
    // =============================================
    composite_literal: $ => prec(PREC.composite_literal, seq(
      field('type', choice(
        $.map_type,
        $.slice_type,
        $.array_type,
        $.struct_type,
        $.identifier,
        $.qualified_type,
        $.generic_type,
      )),
      field('body', $.literal_value),
    )),

    literal_value: $ => seq(
      '{',
      optional(seq(
        choice($.element, $.keyed_element),
        repeat(seq(',', choice($.element, $.keyed_element))),
        optional(','),
      )),
      '}',
    ),

    element: $ => choice($._expression, $.literal_value),

    keyed_element: $ => seq(
      choice(
        seq(field('key', $._expression), ':'),
        seq(field('key', $.literal_value), ':'),
        seq(field('key', $.identifier), ':'),
      ),
      field('value', choice($._expression, $.literal_value)),
    ),

    func_literal: $ => seq(
      'func',
      field('parameters', $.parameter_list),
      optional(field('result', choice($._simple_type, $.parameter_list))),
      field('body', $.block),
    ),

    // =============================================
    // TYPES
    // =============================================
    _type: $ => choice(
      $._simple_type,
      $.parenthesized_type,
    ),

    parenthesized_type: $ => seq('(', $._type, ')'),

    _simple_type: $ => choice(
      $.identifier,
      $.qualified_type,
      $.pointer_type,
      $.array_type,
      $.slice_type,
      $.map_type,
      $.channel_type,
      $.function_type,
      $.struct_type,
      $.interface_type,
      $.generic_type,
    ),

    generic_type: $ => prec.dynamic(1, seq(
      field('type', choice($.identifier, $.qualified_type)),
      field('type_arguments', $.type_arguments),
    )),

    type_arguments: $ => prec.dynamic(1, seq(
      '[',
      commaSep1($._type),
      optional(','),
      ']',
    )),

    pointer_type: $ => prec(PREC.unary, seq('*', $._type)),

    array_type: $ => seq('[', field('length', $._expression), ']', field('element', $._type)),

    slice_type: $ => seq('[', ']', field('element', $._type)),

    map_type: $ => seq('map', '[', field('key', $._type), ']', field('value', $._type)),

    channel_type: $ => choice(
      seq('chan', optional('<-'), $._type),
      seq('<-', 'chan', $._type),
    ),

    qualified_type: $ => seq(
      field('package', $.identifier),
      '.',
      field('name', $.identifier),
    ),

    function_type: $ => seq(
      'func',
      field('parameters', $.parameter_list),
      optional(field('result', choice($._simple_type, $.parameter_list))),
    ),

    struct_type: $ => seq('struct', field('body', $.field_declaration_list)),

    field_declaration_list: $ => seq('{', repeat(seq($.field_declaration, optional(choice(';', '\n')))), '}'),

    field_declaration: $ => seq(
      choice(
        seq(commaSep1(field('name', $.identifier)), field('type', $._type)),
        seq(optional('*'), field('type', choice($.identifier, $.qualified_type))),
      ),
      optional(field('tag', $.raw_string_literal)),
    ),

    interface_type: $ => seq(
      'interface',
      '{',
      repeat(seq(
        choice(
          $.method_spec,
          $.type_elem,
          $.identifier,
          $.qualified_type,
        ),
        optional(choice(';', '\n')),
      )),
      '}',
    ),

    method_spec: $ => seq(
      field('name', $.identifier),
      field('parameters', $.parameter_list),
      optional(field('result', choice($._simple_type, $.parameter_list))),
    ),

    type_elem: $ => seq($._type, repeat(seq('|', $._type))),

    // =============================================
    // DECLARATIONS
    // =============================================
    function_declaration: $ => seq(
      'func',
      field('name', $.identifier),
      optional(field('type_parameters', $.type_parameters)),
      field('parameters', $.parameter_list),
      optional(field('result', choice($._simple_type, $.parameter_list))),
      optional(field('body', $.block)),
    ),

    method_declaration: $ => seq(
      'func',
      field('receiver', $.parameter_list),
      field('name', $.identifier),
      field('parameters', $.parameter_list),
      optional(field('result', choice($._simple_type, $.parameter_list))),
      optional(field('body', $.block)),
    ),

    type_parameters: $ => seq(
      '[',
      commaSep1($.type_parameter),
      optional(','),
      ']',
    ),

    type_parameter: $ => seq(
      field('name', $.identifier),
      optional(field('constraint', $.type_elem)),
    ),

    parameter_list: $ => seq('(', commaSep($.parameter_declaration), optional(','), ')'),

    parameter_declaration: $ => seq(
      optional(field('name', commaSep1($.identifier))),
      optional('...'),
      field('type', $._type),
    ),

    type_declaration: $ => seq(
      'type',
      choice(
        $.type_spec,
        seq('(', repeat(seq($.type_spec, optional(choice(';', '\n')))), ')'),
      ),
    ),

    type_spec: $ => seq(
      field('name', $.identifier),
      optional(field('type_parameters', $.type_parameters)),
      optional('='),
      field('type', $._type),
    ),

    const_declaration: $ => seq(
      'const',
      choice(
        $.const_spec,
        seq('(', repeat(seq($.const_spec, optional(choice(';', '\n')))), ')'),
      ),
    ),

    const_spec: $ => seq(
      field('name', commaSep1($.identifier)),
      optional(seq(
        optional(field('type', $._type)),
        '=',
        field('value', commaSep1($._expression)),
      )),
    ),

    var_declaration: $ => seq(
      'var',
      choice(
        $.var_spec,
        seq('(', repeat(seq($.var_spec, optional(choice(';', '\n')))), ')'),
      ),
    ),

    var_spec: $ => seq(
      field('name', commaSep1($.identifier)),
      choice(
        seq(field('type', $._type), optional(seq('=', field('value', commaSep1($._expression))))),
        seq('=', field('value', commaSep1($._expression))),
      ),
    ),

    short_var_declaration: $ => seq(
      field('left', commaSep1($.identifier)),
      ':=',
      field('right', commaSep1($._expression)),
    ),

    assignment_statement: $ => seq(
      field('left', commaSep1($._expression)),
      field('operator', choice('=', '+=', '-=', '*=', '/=', '%=', '&=', '|=', '^=', '<<=', '>>=')),
      field('right', commaSep1($._expression)),
    ),

    // =============================================
    // STATEMENTS
    // =============================================
    block: $ => seq('{', repeat($._statement), '}'),

    expression_statement: $ => $._expression,

    return_statement: $ => prec.right(seq(
      'return',
      optional(commaSep1($._expression)),
    )),

    if_statement: $ => seq(
      'if',
      optional(seq(field('initializer', $._simple_statement), ';')),
      field('condition', $._expression),
      field('consequence', $.block),
      optional(seq('else', field('alternative', choice($.if_statement, $.block)))),
    ),

    _simple_statement: $ => choice(
      $.expression_statement,
      $.short_var_declaration,
      $.assignment_statement,
    ),

    for_statement: $ => seq(
      'for',
      optional(choice(
        $._expression,
        $.for_clause,
        $.range_clause,
      )),
      field('body', $.block),
    ),

    for_clause: $ => seq(
      optional(field('initializer', $._simple_statement)),
      ';',
      optional(field('condition', $._expression)),
      ';',
      optional(field('update', $._simple_statement)),
    ),

    range_clause: $ => seq(
      optional(seq(
        field('left', commaSep1($.identifier)),
        choice(':=', '='),
      )),
      'range',
      field('right', $._expression),
    ),

    switch_statement: $ => seq(
      'switch',
      optional(seq(field('initializer', $._simple_statement), ';')),
      optional(field('value', $._expression)),
      '{',
      repeat(choice($.expression_case, $.default_case)),
      '}',
    ),

    expression_case: $ => seq(
      'case',
      commaSep1($._expression),
      ':',
      repeat($._statement),
    ),

    default_case: $ => seq('default', ':', repeat($._statement)),

    select_statement: $ => seq('select', '{', repeat($.communication_case), '}'),

    communication_case: $ => seq(
      choice(
        seq('case', choice($.send_statement, $.receive_statement)),
        'default',
      ),
      ':',
      repeat($._statement),
    ),

    send_statement: $ => seq(field('channel', $._expression), '<-', field('value', $._expression)),

    receive_statement: $ => seq(
      optional(seq(commaSep1($.identifier), choice(':=', '='))),
      $._expression,
    ),

    go_statement: $ => seq('go', $._expression),

    defer_statement: $ => seq('defer', $._expression),

    // =============================================
    // IDENTIFIERS
    // =============================================
    identifier: $ => /[a-zA-Z_][a-zA-Z0-9_]*/,
  },
});

// Helpers
function commaSep(rule) {
  return optional(commaSep1(rule));
}

function commaSep1(rule) {
  return seq(rule, repeat(seq(',', rule)));
}
