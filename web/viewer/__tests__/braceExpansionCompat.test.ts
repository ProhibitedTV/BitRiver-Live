type Expand = {
  (pattern: string, options?: { max?: number; maxLength?: number }): string[]
  expand: Expand
}

describe("patched brace-expansion compatibility", () => {
  test("supports legacy callable and current named export shapes", () => {
    // eslint/minimatch still loads the legacy callable CommonJS shape while
    // current consumers use the named `expand` export.
    const legacy = require("brace-expansion") as Expand

    expect(typeof legacy).toBe("function")
    expect(legacy.expand).toBe(legacy)
    expect(legacy("release-{rc.1,stable}")).toEqual([
      "release-rc.1",
      "release-stable",
    ])
  })

  test("delegates length limiting to the patched implementation", () => {
    const legacy = require("brace-expansion") as Expand
    const expanded = legacy("{a,b}".repeat(20), { maxLength: 100 })

    expect(expanded.join("").length).toBeLessThanOrEqual(100)
  })
})
