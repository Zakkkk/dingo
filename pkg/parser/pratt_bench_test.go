package parser

import (
	"testing"

	"github.com/MadAppGang/dingo/pkg/tokenizer"
)

// benchExprs contains sample expressions for benchmarking different parsing paths
var benchExprs = []struct {
	name string
	src  string
}{
	{"simple_ident", "foo"},
	{"binary_add", "a + b"},
	{"binary_chain", "a + b * c - d / e"},
	{"error_prop", "getData()?"},
	{"error_context", `getData() ? "failed"`},
	{"error_lambda_rust", "getData() ? |e| wrap(e)"},
	{"error_lambda_ts", "getData() ? (e) => wrap(e)"},
	{"error_lambda_ts_single", "getData() ? e => wrap(e)"},
	{"ternary", "cond ? trueVal : falseVal"},
	{"nested_ternary", "a ? b : c ? d : e"},
	{"safe_nav", "user?.profile?.name"},
	{"null_coal", "value ?? default"},
	{"match_simple", "match x { Ok(v) => v, Err(e) => 0 }"},
	{"lambda_rust", "|x, y| x + y"},
	{"lambda_ts", "(x, y) => x + y"},
	{"complex_chain", "getData()?.process() ?? defaultValue"},
}

// BenchmarkParseSingleExpression benchmarks parsing individual expressions
func BenchmarkParseSingleExpression(b *testing.B) {
	for _, tc := range benchExprs {
		b.Run(tc.name, func(b *testing.B) {
			src := []byte(tc.src)
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				tok := tokenizer.New(src)
				p := NewPrattParser(tok)
				_ = p.ParseExpression(PrecLowest)
			}
		})
	}
}

// BenchmarkLookahead benchmarks the lookahead functions used for disambiguation
func BenchmarkLookahead(b *testing.B) {
	// Benchmark isTypeScriptLambda lookahead - positive case
	b.Run("isTypeScriptLambda_positive", func(b *testing.B) {
		src := []byte("(x, y) => x + y")
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			tok := tokenizer.New(src)
			p := NewPrattParser(tok)
			_ = p.isTypeScriptLambda()
		}
	})

	// Benchmark isTypeScriptLambda lookahead - negative case
	b.Run("isTypeScriptLambda_negative", func(b *testing.B) {
		src := []byte("(a + b)")
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			tok := tokenizer.New(src)
			p := NewPrattParser(tok)
			_ = p.isTypeScriptLambda()
		}
	})

	// Benchmark question classification - ternary case
	b.Run("classifyQuestion_ternary", func(b *testing.B) {
		src := []byte("trueVal : falseVal")
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			tok := tokenizer.New(src)
			p := NewPrattParser(tok)
			_ = p.classifyQuestionOperator()
		}
	})

	// Benchmark question classification - error propagation postfix
	b.Run("classifyQuestion_postfix", func(b *testing.B) {
		src := []byte(")") // terminator
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			tok := tokenizer.New(src)
			p := NewPrattParser(tok)
			_ = p.classifyQuestionOperator()
		}
	})

	// Benchmark question classification - error with context
	b.Run("classifyQuestion_context", func(b *testing.B) {
		src := []byte(`"error message"`)
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			tok := tokenizer.New(src)
			p := NewPrattParser(tok)
			_ = p.classifyQuestionOperator()
		}
	})

	// Benchmark hasTernaryColon lookahead
	b.Run("hasTernaryColon_found", func(b *testing.B) {
		src := []byte("someValue : other")
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			tok := tokenizer.New(src)
			p := NewPrattParser(tok)
			_ = p.hasTernaryColon()
		}
	})

	// Benchmark hasTernaryColon with nesting
	b.Run("hasTernaryColon_nested", func(b *testing.B) {
		src := []byte("(a, b, c) : other")
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			tok := tokenizer.New(src)
			p := NewPrattParser(tok)
			_ = p.hasTernaryColon()
		}
	})
}

// BenchmarkMatchExpression benchmarks parsing complex match expressions
func BenchmarkMatchExpression(b *testing.B) {
	src := []byte(`match response {
		Ok(data) if data.valid => process(data),
		Ok(data) => default_process(data),
		Err(e) => handle_error(e),
		_ => fallback(),
	}`)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tok := tokenizer.New(src)
		p := NewPrattParser(tok)
		_ = p.ParseExpression(PrecLowest)
	}
}

// BenchmarkLargeFile simulates parsing many expressions
func BenchmarkLargeFile(b *testing.B) {
	// Simulate parsing many expressions
	src := make([]byte, 0, 10000)
	for i := 0; i < 100; i++ {
		src = append(src, []byte("result := getData()? ?? defaultValue\n")...)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(src)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tok := tokenizer.New(src)
		p := NewPrattParser(tok)
		for !p.curTokenIs(tokenizer.EOF) {
			_ = p.ParseExpression(PrecLowest)
			p.nextToken()
		}
	}
}

// BenchmarkParserCreation benchmarks parser allocation overhead
func BenchmarkParserCreation(b *testing.B) {
	src := []byte("foo")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tok := tokenizer.New(src)
		_ = NewPrattParser(tok)
	}
}

// BenchmarkStateOperations benchmarks saveState/restoreState
func BenchmarkStateOperations(b *testing.B) {
	src := []byte("a + b * c")
	tok := tokenizer.New(src)
	p := NewPrattParser(tok)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		state := p.saveState()
		p.nextToken()
		p.restoreState(state)
	}
}

// BenchmarkComplexExpression benchmarks a complex nested expression
func BenchmarkComplexExpression(b *testing.B) {
	// Complex expression combining multiple Dingo features
	src := []byte(`user?.profile?.settings?.getValue("theme") ??
		config?.defaults?.theme ??
		match env {
			"prod" => "light",
			"dev" => "dark",
			_ => "system",
		}`)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		tok := tokenizer.New(src)
		p := NewPrattParser(tok)
		_ = p.ParseExpression(PrecLowest)
	}
}
