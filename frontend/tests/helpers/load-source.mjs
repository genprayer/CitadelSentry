import { readFile } from 'node:fs/promises'
import { createRequire } from 'node:module'
import { dirname, extname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { createContext, SourceTextModule, SyntheticModule } from 'node:vm'
import ts from 'typescript'

const appRoot = fileURLToPath(new URL('../../app/', import.meta.url))
const require = createRequire(import.meta.url)

// Execute the actual TypeScript adapters with an isolated desktop/browser environment.
export async function loadSource(relativePath, globals = {}) {
  const context = createContext({ console, URL, ...globals })
  const modules = new Map()

  async function load(path) {
    if (modules.has(path)) return modules.get(path)
    const module = (async () => {
      const source = await readFile(path, 'utf8')
      const output = ts.transpileModule(source, {
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext }
      }).outputText
      return new SourceTextModule(output, { context, identifier: path })
    })()
    modules.set(path, module)
    return module
  }

  const module = await load(resolve(appRoot, relativePath))
  // The VM owns linking of the whole graph, including shared dependencies.
  await module.link(async (specifier, parent) => {
    if (!specifier.startsWith('~/') && !specifier.startsWith('.')) {
      const key = `external:${specifier}`
      if (modules.has(key)) return modules.get(key)
      const dependency = import(specifier).then(namespace => new SyntheticModule(Object.keys(namespace), function () {
        for (const [name, value] of Object.entries(namespace)) this.setExport(name, value)
      }, { context, identifier: require.resolve(specifier) }))
      modules.set(key, dependency)
      return dependency
    }
    const dependency = specifier.startsWith('~/')
      ? resolve(appRoot, specifier.slice(2))
      : specifier.startsWith('.')
        ? resolve(dirname(parent.identifier), specifier)
        : require.resolve(specifier)
    return load(extname(dependency) ? dependency : `${dependency}.ts`)
  })
  await module.evaluate()
  return module.namespace
}
