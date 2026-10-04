import { readdir, readFile } from 'node:fs/promises';
import { dirname, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import SwaggerParser from '@apidevtools/swagger-parser';
import { Parser } from '@asyncapi/parser';
import { parse } from 'yaml';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
async function documents(directory) {
  const result = [];
  for (const entry of await readdir(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) result.push(...await documents(path));
    else if (/\.(yaml|yml|json)$/.test(entry.name)) result.push(path);
  }
  return result.sort();
}
let failed = false;
let count = 0;
const inputs = await documents(resolve(root, 'api'));
inputs.push(resolve(root, 'docs/api-reference.openapi.json'));
for (const path of inputs) {
  try {
    const source = await readFile(path, 'utf8');
    const document = parse(source);
    if (document?.openapi) {
      await SwaggerParser.validate(path);
      count += 1;
    } else if (document?.asyncapi) {
      const diagnostics = await new Parser().validate(source, { source: pathToFileURL(path).href });
      const errors = diagnostics.filter(diagnostic => diagnostic.severity === 0);
      if (errors.length) {
        throw new Error(errors.map(diagnostic => `${diagnostic.path.join('.')}: ${diagnostic.message}`).join('\n'));
      }
      count += 1;
    }
  } catch (error) {
    failed = true;
    console.error(`${path}: ${error.message}`);
  }
}
if (failed) process.exitCode = 1;
else console.log(`Validated ${count} complete API documents and their references.`);
