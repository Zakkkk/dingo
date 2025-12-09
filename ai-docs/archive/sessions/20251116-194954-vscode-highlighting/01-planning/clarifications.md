# User Clarifications for VSCode Highlighting

## Marker Style Decision
**Block-level markers only** - All generated code wrapped with START/END comments.
- Cleaner than line-by-line markers
- Example:
  ```go
  // DINGO:GENERATED:START error_propagation
  __tmp0, __err0 := fetchUser(id)
  if __err0 != nil {
      return User{}, __err0
  }
  // DINGO:GENERATED:END
  ```

## Visual Style Decision
**Configurable in plugin settings** - Users can choose between:
- Subtle: Light background color only (default)
- Bold: Background color + border

This gives users flexibility based on their preference and theme.

## Transpiler Default
**Enabled by default** - Marker generation is on by default
- Markers are just comments, harmless in generated code
- VSCode highlighting works immediately without configuration
- Users can disable with a flag if desired

## Syntax Highlighting Priorities
**All improvements** - Implement everything we have:
1. ✅ Error messages in `expr? "message"` - HIGH priority
2. ✅ Generated variable patterns (`__err0`, `__tmp0`) - MEDIUM priority
3. ✅ Result/Option types (`Result[T,E]`, `Option[T]`) - MEDIUM priority
4. ✅ Error propagation operator (`?`) - Already working, enhance if needed
5. ✅ "Everything we have for now" - All current Dingo syntax features

## Implementation Notes
- VSCode extension will have settings panel for visual customization
- Default style will be "subtle" for non-intrusive highlighting
- Users can toggle to "bold" via VSCode settings
- Possibly add "outline" and "disabled" options as well
