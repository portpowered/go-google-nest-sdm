import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import { GoGenerator, ConstrainedStringModel, RenderOutput } from '@asyncapi/modelina';
import { parse } from 'yaml';

const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const document = parse(await readFile(resolve(root, 'api/asyncapi.yaml'), 'utf8'));
const schemas = structuredClone(document.components.schemas);
// OpenAPI 3.0 uses one schema example; the AsyncAPI message retains the complete
// named example catalog. Keep that dialect conversion in the generated view.
function openapiProjection(value) {
  if (Array.isArray(value)) return value.map(openapiProjection);
  if (!value || typeof value !== 'object') return value;
  const result = Object.fromEntries(Object.entries(value).map(([key, child]) => [key, openapiProjection(child)]));
  if (Array.isArray(result.examples)) {
    result.example = result.examples[0];
    delete result.examples;
  }
  return result;
}
// A generated OpenAPI view lets public operation schemas reference event types
// without teaching the REST generator the AsyncAPI document dialect.
await writeFile(resolve(root, 'api/events.openapi.yaml'), JSON.stringify({
  openapi: '3.0.3', info: { title: 'Generated SDM event projection', version: document.info.version },
  paths: {}, components: { schemas: openapiProjection(schemas) },
}, null, 2) + '\n');
// Modelina consumes a Draft 7 schema projection of the signaling catalog.
// Traits is generated separately by oapi-codegen from the canonical trait catalog.
function project(value) {
  if (Array.isArray(value)) return value.map(project);
  if (!value || typeof value !== 'object') return value;
  const result = Object.fromEntries(Object.entries(value).map(([key, child]) => [key, project(child)]));
  if (result.$ref?.startsWith('#/components/schemas/')) result.$ref = result.$ref.replace('#/components/schemas/', '#/definitions/');
  if (result.$ref?.startsWith('./traits.openapi.yaml#')) result.$ref = '#/definitions/Traits';
  if (result['x-known-values']) result.enum = result['x-known-values'];
  // Requirement-only alternatives constrain object presence, not its Go shape.
  // The canonical schemas and generated codecs retain these constraints.
  if (result.anyOf?.every(branch => Object.keys(branch).length === 1 && Array.isArray(branch.required))) delete result.anyOf;
  return result;
}
const definitions = Object.fromEntries(Object.entries(schemas).map(([name, schema]) => [name, project(schema)]));
definitions.Traits = { title: 'Traits', type: 'object', additionalProperties: true };
const fieldName = field => {
  const wire = field.unconstrainedPropertyName;
  if (wire.startsWith('sdm.devices.events.')) return wire.split('.')[3];
  return field.unconstrainedPropertyName === 'type' ? 'Type' : field.propertyName;
};
function fieldType(field) {
  const property = field.property;
  let type = field.unconstrainedPropertyName === 'traits' ? 'Traits' : property.type.replace(/^\*/, '');
  if (scalarNames.includes(property.originalInput?.title)) type = property.originalInput.title;
  if (property.originalInput?.format === 'date-time') type = 'time.Time';
  return field.required ? type : `*${type}`;
}
function codecs(model, fields) {
  const name = model.name;
  const known = fields.map(field => JSON.stringify(field.unconstrainedPropertyName));
  const required = fields.filter(field => field.required).map(field => JSON.stringify(field.unconstrainedPropertyName));
  const requireChecks = required.map(key => `if value, present := properties[${key}]; !present || string(value) == "null" { return fmt.Errorf("${name}: required field %s is missing or null", ${key}) }`).join('\n');
  const nullChecks = known.map(key => `if value, present := properties[${key}]; present && string(value) == "null" { return fmt.Errorf("${name}: field %s cannot be null", ${key}) }`).join('\n');
  const alternatives = schemas[name]?.anyOf;
  const alternativeCheck = alternatives?.every(branch => Object.keys(branch).length === 1 && Array.isArray(branch.required))
    ? `if !(${alternatives.map(branch => '(' + branch.required.map(key => `len(properties[${JSON.stringify(key)}]) != 0`).join(' && ') + ')').join(' || ')}) { return fmt.Errorf("${name}: missing required update variant") }` : '';
  const deletes = known.map(key => `delete(properties, ${key})`).join('\n');
  // Required keys must win even if users insert a conflicting extension key.
  const marshalDeletes = known.map(key => `delete(properties, ${key})`).join('\n');
  return `
// UnmarshalJSON decodes known fields and preserves future server fields.
func (value *${name}) UnmarshalJSON(data []byte) error {
  var properties map[string]json.RawMessage
  if err := json.Unmarshal(data, &properties); err != nil { return err }
  if properties == nil { return fmt.Errorf("${name}: expected an object") }
  ${requireChecks}
  ${nullChecks}
  ${alternativeCheck}
  type known ${name}
  var decoded known
  if err := json.Unmarshal(data, &decoded); err != nil { return err }
  ${deletes}
  decoded.AdditionalProperties = properties
  *value = ${name}(decoded)
  return nil
}

// MarshalJSON combines typed fields with preserved future server fields.
func (value ${name}) MarshalJSON() ([]byte, error) {
  type known ${name}
  data, err := json.Marshal(known(value))
  if err != nil { return nil, err }
  properties := make(map[string]json.RawMessage, len(value.AdditionalProperties))
  for key, raw := range value.AdditionalProperties { properties[key] = raw }
  ${marshalDeletes}
  var typed map[string]json.RawMessage
  if err := json.Unmarshal(data, &typed); err != nil { return nil, err }
  for key, raw := range typed { properties[key] = raw }
  return json.Marshal(properties)
}`;
}
const preset = {
  struct: {
    self({ model }) {
      const fields = Object.values(model.properties).filter(field => field.unconstrainedPropertyName !== 'additionalProperties');
      const fieldsCode = fields.map(field => {
        const desc = field.property.originalInput?.description || `The ${field.unconstrainedPropertyName} value.`;
        return `// ${fieldName(field)} ${desc}\n${fieldName(field)} ${fieldType(field)} \`json:"${field.unconstrainedPropertyName}${field.required ? '' : ',omitempty'}"\``;
      }).join('\n');
      return `// ${model.name} ${model.originalInput.description || 'is a typed SDM event payload.'}\ntype ${model.name} struct {\n${fieldsCode}\n// AdditionalProperties retains unrecognized JSON fields.\nAdditionalProperties map[string]json.RawMessage \`json:"-"\`\n}\n${codecs(model, fields)}`;
    },
  },
  enum: {
    self({ model }) {
      const values = model.values.map(item => `// ${model.name}${JSON.parse(item.value)} is the ${JSON.parse(item.value)} value.\n${model.name}${JSON.parse(item.value)} ${model.name} = ${item.value}`).join('\n');
      return `// ${model.name} is an extensible protocol string.\ntype ${model.name} string\nconst (\n${values}\n)`;
    },
  },
};
// Modelina's default Go renderer only names objects and enums. Extend its
// primitive renderer so semantic identifiers retain distinct Go types.
const scalarNames = Object.entries(schemas).filter(([, schema]) => schema.type === 'string' && !schema['x-known-values']).map(([name]) => name);
class EventGenerator extends GoGenerator {
  async render(args) {
    const model = args.constrainedModel;
    const name = model.originalInput?.title;
    if (model instanceof ConstrainedStringModel && scalarNames.includes(name)) {
      return RenderOutput.toRenderOutput({
        result: `// ${name} ${model.originalInput.description}\ntype ${name} string`,
        renderedName: name, dependencies: [],
      });
    }
    return super.render(args);
  }
}
const generator = new EventGenerator({ presets: [preset] });
const primitives = (await Promise.all(scalarNames.map(name => generator.generate(definitions[name])))).flat();
const generated = await generator.generate({ $schema: 'http://json-schema.org/draft-07/schema#', ...definitions.EventEnvelope, definitions });
const results = [...primitives, ...generated].filter(model => Object.hasOwn(schemas, model.modelName)).sort((a, b) => a.modelName.localeCompare(b.modelName)).map(model => model.result);
for (const packageName of ['dependencymodels', 'sdm']) {
  const directory = resolve(root, 'pkg', packageName);
  await mkdir(directory, { recursive: true });
  const output = resolve(directory, 'events.gen.go');
  await writeFile(output, `// Code generated by @asyncapi/modelina 5.10.1 from api/asyncapi.yaml. DO NOT EDIT.\n\npackage ${packageName}\n\nimport ("encoding/json"; "fmt"; "time")\n\n${results.join('\n\n')}\n`);
  execFileSync('gofmt', ['-w', output]);
}
