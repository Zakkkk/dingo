// Tree-sitter grammar for Dingo language
// Dingo is a superset of Go with additional syntax sugar

module.exports = grammar({
  name: 'dingo',

  extras: $ => [
    /\s/,
    $.comment,
  ],

  inline: $ => [
    $._type,
    $._statement,
    $._expression,
  ],

  word: $ => $.identifier,

  conflicts: $ => [
    // Lambda |x| vs OR ||
    [$.lambda_expression, $.binary_expression],
    // Generics vs comparison: foo<T> vs foo < T
    [$.type_arguments, $.binary_expression],
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
      $.type_declaration,
      $.const_declaration,
      $.var_declaration,
      $.let_declaration,        // Dingo: let binding
      $.enum_declaration,       // Dingo: enum
      $.expression_statement,
      $.return_statement,
      $.if_statement,
      $.for_statement,
      $.block,
    ),

    package_clause: $ => seq('package', $.identifier),

    import_declaration: $ => seq(
      'import',
      choice(
        $.import_spec,
        seq('(', repeat($.import_spec), ')'),
      ),
    ),

    import_spec: $ => seq(
      optional(field('alias', $.identifier)),
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
    ),

    enum_variant_fields: $ => seq(
      '{',
      commaSep($.enum_field),
      optional(','),
      '}',
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
      $.identifier_pattern,
      $.variant_pattern,
      $.literal_pattern,
    ),

    wildcard_pattern: $ => '_',

    identifier_pattern: $ => $.identifier,

    variant_pattern: $ => seq(
      field('name', $.identifier),
      optional(seq('(', commaSep($.pattern), ')')),
    ),

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
    rust_style_lambda: $ => seq(
      '|',
      commaSep($.lambda_parameter),
      '|',
      field('body', choice($._expression, $.block)),
    ),

    // TypeScript-style: (x) => expr or x => expr
    arrow_style_lambda: $ => seq(
      choice(
        seq('(', commaSep($.lambda_parameter), ')'),
        $.identifier,  // Single param without parens
      ),
      '=>',
      field('body', choice($._expression, $.block)),
    ),

    lambda_parameter: $ => seq(
      field('name', $.identifier),
      optional(seq(':', field('type', $._type))),
    ),

    // =============================================
    // DINGO: SPECIAL OPERATORS
    // =============================================

    // Error propagation: expr?
    error_propagation: $ => prec.left(15, seq(
      field('operand', $._expression),
      '?',
    )),

    // Safe navigation: a?.b
    safe_navigation: $ => prec.left(14, seq(
      field('operand', $._expression),
      '?.',
      field('field', $.identifier),
    )),

    // Null coalescing: a ?? b
    null_coalesce: $ => prec.left(3, seq(
      field('left', $._expression),
      '??',
      field('right', $._expression),
    )),

    // =============================================
    // EXPRESSIONS
    // =============================================
    _expression: $ => choice(
      $.identifier,
      $.int_literal,
      $.float_literal,
      $.interpreted_string_literal,
      $.raw_string_literal,
      $.rune_literal,
      $.true,
      $.false,
      $.nil,
      $.composite_literal,
      $.function_literal,
      $.lambda_expression,         // Dingo
      $.match_expression,          // Dingo
      $.error_propagation,         // Dingo
      $.safe_navigation,           // Dingo
      $.null_coalesce,             // Dingo
      $.unary_expression,
      $.binary_expression,
      $.selector_expression,
      $.index_expression,
      $.call_expression,
      $.parenthesized_expression,
    ),

    binary_expression: $ => choice(
      ...[
        ['||', 1],
        ['&&', 2],
        // ?? is handled separately as null_coalesce
        ['==', 4],
        ['!=', 4],
        ['<', 5],
        ['<=', 5],
        ['>', 5],
        ['>=', 5],
        ['+', 6],
        ['-', 6],
        ['|', 6],
        ['^', 6],
        ['*', 7],
        ['/', 7],
        ['%', 7],
        ['<<', 7],
        ['>>', 7],
        ['&', 7],
        ['&^', 7],
      ].map(([op, prec_val]) =>
        prec.left(prec_val, seq(
          field('left', $._expression),
          field('operator', op),
          field('right', $._expression),
        ))
      ),
    ),

    unary_expression: $ => prec.left(8, seq(
      choice('!', '-', '+', '^', '*', '&', '<-'),
      $._expression,
    )),

    selector_expression: $ => prec.left(13, seq(
      field('operand', $._expression),
      '.',
      field('field', $.identifier),
    )),

    index_expression: $ => prec.left(13, seq(
      field('operand', $._expression),
      '[',
      field('index', $._expression),
      ']',
    )),

    call_expression: $ => prec.left(13, seq(
      field('function', $._expression),
      optional($.type_arguments),
      '(',
      optional(commaSep($._expression)),
      ')',
    )),

    parenthesized_expression: $ => seq('(', $._expression, ')'),

    // =============================================
    // TYPES
    // =============================================
    _type: $ => choice(
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

    generic_type: $ => seq(
      $.identifier,
      $.type_arguments,
    ),

    type_arguments: $ => seq(
      '[',
      commaSep1($._type),
      ']',
    ),

    type_parameters: $ => seq(
      '[',
      commaSep1($.type_parameter),
      ']',
    ),

    type_parameter: $ => seq(
      $.identifier,
      optional($.type_constraint),
    ),

    type_constraint: $ => $._type,

    pointer_type: $ => seq('*', $._type),
    array_type: $ => seq('[', $._expression, ']', $._type),
    slice_type: $ => seq('[', ']', $._type),
    map_type: $ => seq('map', '[', $._type, ']', $._type),
    channel_type: $ => choice(
      seq('chan', $._type),
      seq('chan', '<-', $._type),
      seq('<-', 'chan', $._type),
    ),

    qualified_type: $ => seq($.identifier, '.', $.identifier),

    function_type: $ => seq(
      'func',
      $.parameter_list,
      optional(choice($._type, $.parameter_list)),
    ),

    struct_type: $ => seq('struct', '{', repeat($.field_declaration), '}'),

    interface_type: $ => seq('interface', '{', repeat($.method_spec), '}'),

    // =============================================
    // DECLARATIONS
    // =============================================
    function_declaration: $ => seq(
      'func',
      optional($.receiver),
      field('name', $.identifier),
      optional($.type_parameters),
      $.parameter_list,
      optional(field('result', choice($._type, $.parameter_list))),
      optional($.block),
    ),

    receiver: $ => seq('(', $.identifier, optional('*'), $._type, ')'),

    parameter_list: $ => seq('(', commaSep($.parameter), ')'),

    parameter: $ => seq(
      optional(field('name', $.identifier)),
      optional('...'),
      field('type', $._type),
    ),

    type_declaration: $ => seq('type', $.type_spec),

    type_spec: $ => seq(
      field('name', $.identifier),
      optional($.type_parameters),
      field('type', $._type),
    ),

    const_declaration: $ => seq(
      'const',
      choice(
        $.const_spec,
        seq('(', repeat($.const_spec), ')'),
      ),
    ),

    const_spec: $ => seq(
      field('name', $.identifier),
      optional(seq(':', $._type)),
      optional(seq('=', $._expression)),
    ),

    var_declaration: $ => seq(
      'var',
      choice(
        $.var_spec,
        seq('(', repeat($.var_spec), ')'),
      ),
    ),

    var_spec: $ => seq(
      field('name', $.identifier),
      choice(
        seq(field('type', $._type), optional(seq('=', $._expression))),
        seq('=', $._expression),
      ),
    ),

    // =============================================
    // OTHER STATEMENTS
    // =============================================
    block: $ => seq('{', repeat($._statement), '}'),

    expression_statement: $ => $._expression,

    return_statement: $ => seq('return', optional($._expression)),

    if_statement: $ => seq(
      'if',
      optional(seq($.short_var_declaration, ';')),
      field('condition', $._expression),
      field('consequence', $.block),
      optional(seq('else', choice($.if_statement, $.block))),
    ),

    short_var_declaration: $ => seq(
      $.identifier,
      ':=',
      $._expression,
    ),

    for_statement: $ => seq('for', optional($._expression), $.block),

    field_declaration: $ => seq($.identifier, $._type),

    method_spec: $ => seq($.identifier, $.parameter_list),

    composite_literal: $ => seq(
      $._type,
      '{',
      optional(commaSep($.keyed_element)),
      '}',
    ),

    keyed_element: $ => seq(
      optional(seq(choice($.identifier, $._expression), ':')),
      $._expression,
    ),

    function_literal: $ => seq(
      'func',
      $.parameter_list,
      optional($._type),
      $.block,
    ),

    // =============================================
    // LITERALS
    // =============================================
    identifier: $ => /[a-zA-Z_][a-zA-Z0-9_]*/,

    int_literal: $ => choice(
      /0[xX][0-9a-fA-F]+/,    // hex
      /0[oO][0-7]+/,          // octal
      /0[bB][01]+/,           // binary
      /[0-9]+/,               // decimal
    ),

    float_literal: $ => /[0-9]+\.[0-9]+([eE][+-]?[0-9]+)?/,

    interpreted_string_literal: $ => seq(
      '"',
      repeat(choice(
        token.immediate(/[^"\\]+/),
        $.escape_sequence,
      )),
      '"',
    ),

    raw_string_literal: $ => /`[^`]*`/,

    rune_literal: $ => seq("'", choice(/[^'\\]/, $.escape_sequence), "'"),

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
  },
});

// Helpers
function commaSep(rule) {
  return optional(commaSep1(rule));
}

function commaSep1(rule) {
  return seq(rule, repeat(seq(',', rule)));
}
