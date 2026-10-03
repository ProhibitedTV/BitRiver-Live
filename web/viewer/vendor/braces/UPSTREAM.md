# Temporary braces security fork

Owner: BitRiver Live maintainers. Review/removal target: 2026-10-16.

This is a private, MIT-licensed fork of `braces@3.0.3`, installed under the
`braces` dependency key as `@bitriver/braces@3.0.3-bitriver.1`. The npm override
also redirects ESLint's `fast-glob -> micromatch -> braces` dependency here.
It is development tooling, not a viewer runtime dependency.

Source: <https://github.com/micromatch/braces/tree/3.0.3> and
<https://registry.npmjs.org/braces/-/braces-3.0.3.tgz>.
The upstream tarball integrity is
`sha512-yQbXgO/OSZVD2IsiLlro+7Hf6Q18EJrKSEsdoMzKePKXct3gvD8oLcOQdIzGupr5Fj+EDe8gO/lxc1BzfMpxvA==`.
The original license and source are retained; only the changes below were added.

## Mitigation

As of 2026-10-03, [GHSA-vfj7-8cjw-p6xm](https://github.com/advisories/GHSA-vfj7-8cjw-p6xm)
has no patched npm release. The original parser can create deeply nested trees
that overflow recursive walkers even below its 10,000-character input limit.

- `lib/guard.js` adds a maximum syntax nesting depth of 64.
- `lib/parse.js` checks depth before opening braces or parentheses, preserving
  literal/escaped/quoted and character-class parsing.
- `compile`, `expand`, and `stringify` validate externally supplied ASTs with
  an iterative traversal before entering recursive upstream code. Invalid or
  cyclic trees, parent chains longer than 65, and trees over 30,000 nodes fail.
- Excess depth raises a deliberate `SyntaxError`, not a stack-overflow error.
  Malformed/cyclic ASTs raise `TypeError`.

These limits deliberately reject exceptionally deep globs. Existing expansion
range limits remain unchanged; this is a nesting mitigation, not a promise
that every possible expansion is inexpensive. Regression coverage lives in
`__tests__/bracesSecurity.test.ts`, alongside brace-expansion compatibility tests.

An audit reporting zero findings after the local package substitution is not
independent verification of the patch. Review the implementation and attack
tests as well. Remove the fork only after adopting an upstream fix, removing
the override/direct dependency together, and passing clean install, audit,
glob regression, viewer integration, and Docker gates.
