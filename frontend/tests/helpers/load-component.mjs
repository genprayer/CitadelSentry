import { readFile } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import { createContext, SourceTextModule, SyntheticModule } from 'node:vm'
import { compileScript, parse } from 'vue/compiler-sfc'
import ts from 'typescript'
import * as vue from 'vue'

// Execute the actual setup script with Vue reactivity and explicit dependency fixtures.
export async function loadComponent(relativePath, dependencies, globals = {}) {
  const filename = fileURLToPath(new URL(`../../app/${relativePath}`, import.meta.url))
  const { descriptor } = parse(await readFile(filename, 'utf8'), { filename })
  const javascript = ts.transpileModule(descriptor.scriptSetup.content, {
    compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext }
  }).outputText
  const executable = parse(`<script setup>${javascript}</script>`, { filename }).descriptor
  const compiled = compileScript(executable, { id: 'component-test' })
  const context = createContext({ console, ...vue, ...globals })
  const module = new SourceTextModule(compiled.content, { context, identifier: filename })
  await module.link(async name => {
    const namespace = name === 'vue' ? vue : dependencies[name]
    if (!namespace) throw new Error(`Missing explicit component dependency: ${name}`)
    return new SyntheticModule(Object.keys(namespace), function () {
      for (const [key, value] of Object.entries(namespace)) this.setExport(key, value)
    }, { context, identifier: name })
  })
  await module.evaluate()
  return module.namespace.default
}
