'use strict'

// CI synchronize marker; reverted in the next commit.
const fs = require('node:fs')
const path = require('node:path')

const expectedVersion = '5.0.9'
const marker = '// BitRiver legacy CommonJS compatibility'

function* packageDirectories(nodeModulesDirectory) {
  if (!fs.existsSync(nodeModulesDirectory)) return

  for (const entry of fs.readdirSync(nodeModulesDirectory, {
    withFileTypes: true,
  })) {
    if (!entry.isDirectory() || entry.name === '.bin') continue

    const entryPath = path.join(nodeModulesDirectory, entry.name)
    if (entry.name.startsWith('@')) {
      for (const scopedEntry of fs.readdirSync(entryPath, {
        withFileTypes: true,
      })) {
        if (!scopedEntry.isDirectory()) continue
        const packagePath = path.join(entryPath, scopedEntry.name)
        yield packagePath
        yield* packageDirectories(path.join(packagePath, 'node_modules'))
      }
      continue
    }

    yield entryPath
    yield* packageDirectories(path.join(entryPath, 'node_modules'))
  }
}

const viewerRoot = path.resolve(__dirname, '..', '..')
const packagePaths = [...packageDirectories(path.join(viewerRoot, 'node_modules'))]
  .filter((packagePath) => path.basename(packagePath) === 'brace-expansion')

if (packagePaths.length === 0) {
  throw new Error('brace-expansion is not installed')
}

for (const packagePath of packagePaths) {
  const manifestPath = path.join(packagePath, 'package.json')
  const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'))
  if (manifest.version !== expectedVersion) {
    throw new Error(
      `refusing to patch brace-expansion ${manifest.version}; expected ${expectedVersion}`,
    )
  }

  const commonJsPath = path.join(packagePath, 'dist', 'commonjs', 'index.js')
  let source = fs.readFileSync(commonJsPath, 'utf8')
  if (!source.includes(marker)) {
    source += [
      '',
      marker,
      '// minimatch <=9 expects require("brace-expansion") to be callable.',
      '// Keep named properties for current CommonJS consumers.',
      'const bitriverLegacyExpand = module.exports.expand;',
      'Object.assign(bitriverLegacyExpand, module.exports);',
      'module.exports = bitriverLegacyExpand;',
      '',
    ].join('\n')
    fs.writeFileSync(commonJsPath, source)
  }

  delete require.cache[commonJsPath]
  const loaded = require(commonJsPath)
  if (typeof loaded !== 'function' || loaded.expand !== loaded) {
    throw new Error(`brace-expansion compatibility check failed at ${packagePath}`)
  }
}

console.log(
  `patched ${packagePaths.length} brace-expansion ${expectedVersion} CommonJS install(s)`,
)
