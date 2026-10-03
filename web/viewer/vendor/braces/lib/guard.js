'use strict';

// Bound recursion before upstream parser/walkers touch attacker-controlled trees.
const MAX_DEPTH = 64;
const MAX_NODES = 30000;

const assertDepth = depth => {
  if (depth > MAX_DEPTH) {
    throw new SyntaxError(`Brace pattern exceeds maximum nesting depth (${MAX_DEPTH})`);
  }
};

const assertAst = ast => {
  const pending = [{ node: ast, depth: 0 }];
  const seen = new Set();
  while (pending.length > 0) {
    const { node, depth } = pending.pop();
    if (!node || typeof node !== 'object' || seen.has(node)) {
      throw new TypeError('Invalid or cyclic braces AST');
    }
    seen.add(node);
    if (seen.size > MAX_NODES) {
      throw new SyntaxError('Braces AST exceeds maximum node count');
    }
    // Parent links are consumed by expand(), but are not child-tree edges.
    let parent = node.parent;
    let parentDepth = 0;
    while (parent) {
      if (++parentDepth > MAX_DEPTH + 1) {
        throw new TypeError('Invalid or deeply nested braces AST parent chain');
      }
      parent = parent.parent;
    }
    if (node.nodes) {
      assertDepth(depth);
      if (!Array.isArray(node.nodes)) {
        throw new TypeError('Invalid braces AST children');
      }
      if (seen.size + pending.length + node.nodes.length > MAX_NODES) {
        throw new SyntaxError('Braces AST exceeds maximum node count');
      }
      for (const child of node.nodes) {
        pending.push({ node: child, depth: depth + 1 });
      }
    }
  }
};

module.exports = { assertDepth, assertAst };
