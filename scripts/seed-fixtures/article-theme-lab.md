## Theme lab

This article exercises **headings**, lists, inline `code`, blockquotes, tables, callouts, and a code fence.

### Typography checklist

- Serif display for titles
- Sans body text at comfortable line height
- Muted meta lines for dates and kinds
- Accent color on links and labels

> Themes are not just background colors — spacing and borders matter too.

```js
const theme = document.documentElement.dataset.theme
console.log(`active theme: ${theme}`)
```

| Element | Token |
|---------|-------|
| Page bg | `--bg` |
| Cards | `--surface` |
| Links | `--accent` |

> [!NOTE]
> Callouts should pick up per-theme alert backgrounds.

> [!TIP]
> Switch presets in **Admin → Appearance** before saving.

> [!IMPORTANT]
> Preview is isolated until you click **Save theme**.

> [!WARNING]
> Hard-coded hex colors break dark mode.

> [!CAUTION]
> Always verify markdown alerts after token refactors.
