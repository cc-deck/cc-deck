# Research: Config Validation

## Unicode East Asian Width in Go

**Decision**: Use Go's `unicode` stdlib package for East Asian Width property lookup.

**Rationale**: The `unicode` package provides `unicode.Is(rangeTable, rune)` for testing character properties. East Asian Width categories (Wide, Ambiguous, Narrow) can be defined as range tables derived from the Unicode Character Database. This avoids external dependencies and keeps the validation self-contained.

**Alternatives considered**:
- `golang.org/x/text/width`: Provides `width.LookupRune()` which returns Width properties. More ergonomic but adds a dependency. The stdlib approach is sufficient for our needs (checking a few characters per badge value).
- Hardcoded codepoint ranges: Brittle, would need updates with each Unicode version. Rejected.

**Implementation approach**: Define range tables for Wide and Ambiguous characters covering the codepoints commonly used in terminal badge icons (CJK Symbols, Miscellaneous Symbols, Geometric Shapes, Trigrams, Emoji). The `unicode/utf8` package handles multi-byte decoding.

**Emoji detection**: Emoji are identified by checking against `unicode.Is(unicode.So, r)` (Symbol, other) combined with width > 1, or by checking specific emoji ranges (U+1F000-U+1FFFF, U+2600-U+27BF with emoji presentation). The practical approach: check if a rune has `width.Kind` of `EastAsianWide` or `EastAsianFullwidth`, or is in emoji ranges.

## Finding Struct Design

**Decision**: Use a simple struct with string fields for severity, category, message, and suggestion.

**Rationale**: A struct with typed severity (const string) and category (const string) is simple, testable, and sufficient. No need for an interface hierarchy or error wrapping.

**Design**:
```go
type Severity string
const (
    SeverityError   Severity = "error"
    SeverityWarning Severity = "warning"
)

type Category string
const (
    CategoryBadges    Category = "badges"
    CategoryProfiles  Category = "profiles"
    CategoryVoice     Category = "voice"
    CategoryStructure Category = "structure"
)

type Finding struct {
    Severity   Severity
    Category   Category
    Message    string
    Suggestion string
}
```

## Load-Time Integration

**Decision**: Add a `ValidateAndWarn()` helper that calls `Validate()` and prints a one-line summary to stderr using `fmt.Fprintf(os.Stderr, ...)`.

**Rationale**: The `Load()` function returns `(*Config, error)` and is called by many commands. Adding validation output directly in `Load()` would mix concerns. A separate `ValidateAndWarn()` function can be called after `Load()` at the CLI level (in the root command's PersistentPreRun or in each command that loads config).

**Integration point**: The `cmd_context()` function or a shared `loadConfig()` helper used by commands that read config. This keeps validation opt-in per command group rather than global.

## Badge Color Prefix Parsing

**Decision**: Reuse the same `#RRGGBB:icon` parsing logic as the Rust plugin's `parse_badge_color()`.

**Rationale**: The validator must extract the icon portion from badge values that may include a color prefix. The format is `#RRGGBB:icon` where the hex portion is exactly 6 characters. The Go implementation mirrors the Rust plugin's existing parser.

**Validation checks**:
1. If badge value starts with `#` and contains `:`, extract hex portion
2. Hex must be exactly 6 characters of valid hex digits
3. Icon is everything after the first `:`
