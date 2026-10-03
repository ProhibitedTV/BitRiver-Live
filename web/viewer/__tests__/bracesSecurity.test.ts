type Ast = { type: string; nodes?: Ast[]; parent?: Ast; value?: string }
type Braces = {
  (pattern: string, options?: Record<string, unknown>): string[]
  parse(pattern: string): Ast
  compile(pattern: string | Ast): string
  expand(pattern: string | Ast, options?: Record<string, unknown>): string[]
  stringify(pattern: string | Ast): string
  create(pattern: string): string[]
}

const braces = require("braces") as Braces
const nested = (depth: number, open = "{", close = "}") =>
  open.repeat(depth) + "a,b" + close.repeat(depth)

describe("depth-guarded braces tooling dependency", () => {
  test("installs the documented local fork", () => {
    expect(require("braces/package.json").name).toBe("@bitriver/braces")
  })

  test.each(["parse", "compile", "expand", "stringify", "create"] as const)(
    "%s rejects deeply nested input before recursive walkers",
    (method) => {
      // Below upstream's 10,000-character cap, but enough to exhaust walkers.
      expect(() => braces[method](nested(4096))).toThrow(SyntaxError)
      expect(() => braces[method](nested(4096))).toThrow(/maximum nesting depth/)
    },
  )

  test("bounds parentheses, mixed nesting, and unclosed blocks", () => {
    expect(() => braces.parse(nested(65, "(", ")"))).toThrow(SyntaxError)
    expect(() => braces.parse("{(".repeat(33) + "x" + ")}".repeat(33))).toThrow(SyntaxError)
    expect(() => braces.parse("{".repeat(65))).toThrow(SyntaxError)
    expect(() => braces(nested(65))).toThrow(SyntaxError)
  })

  test("accepts the supported boundary and preserves literal syntax", () => {
    const boundary = nested(64)
    expect(braces.stringify(braces.parse(boundary))).toBe(boundary)
    expect(() => braces.compile(boundary)).not.toThrow()
    expect(() => braces.expand(boundary)).not.toThrow()
    const escaped = "\\{".repeat(100) + "x" + "\\}".repeat(100)
    expect(() => braces.parse(escaped)).not.toThrow()
    expect(() => braces.parse('"' + nested(100) + '"')).not.toThrow()
    expect(() => braces.parse("[" + nested(100) + "]")).not.toThrow()
  })

  test.each(["compile", "expand", "stringify"] as const)(
    "%s rejects unsafe externally supplied ASTs",
    (method) => {
      const deep: Ast = { type: "root", nodes: [] }
      let cursor = deep
      for (let i = 0; i < 66; i++) {
        const child: Ast = { type: "brace", nodes: [] }
        cursor.nodes!.push(child)
        cursor = child
      }
      expect(() => braces[method](deep)).toThrow(SyntaxError)
      const cyclic: Ast = { type: "root", nodes: [] }
      cyclic.nodes!.push(cyclic)
      expect(() => braces[method](cyclic)).toThrow(TypeError)
      const parentCycle: Ast = { type: "root", nodes: [] }
      parentCycle.parent = parentCycle
      expect(() => braces[method](parentCycle)).toThrow(TypeError)
      const wide: Ast = { type: "root", nodes: Array.from({ length: 30001 }, () => ({ type: "text", value: "a" })) }
      expect(() => braces[method](wide)).toThrow(/maximum node count/)
    },
  )

  test("preserves alternatives, padded ranges, options, and transitive glob matching", () => {
    expect(braces.compile("a{b,c}d")).toBe("a(b|c)d")
    expect(braces.expand("file-{01..03}.ts")).toEqual(["file-01.ts", "file-02.ts", "file-03.ts"])
    expect(braces.expand("{a,a,}", { nodupes: true, noempty: true })).toEqual(["a"])
    expect(() => braces.expand("{1..1001}", { rangeLimit: 1000 })).toThrow(RangeError)
    const micromatch = require("micromatch") as { isMatch(path: string, pattern: string): boolean }
    expect(micromatch.isMatch("src/app/page.tsx", "src/{app,components}/**/*.{ts,tsx}")).toBe(true)
    expect(micromatch.isMatch("src/private/key.txt", "src/{app,components}/**/*.{ts,tsx}")).toBe(false)
    const fastGlob = require("fast-glob") as { sync(pattern: string[], options: { cwd: string }): string[] }
    expect(fastGlob.sync(["vendor/braces/{index,lib/guard}.js"], { cwd: process.cwd() }).sort())
      .toEqual(["vendor/braces/index.js", "vendor/braces/lib/guard.js"])
  })
})
