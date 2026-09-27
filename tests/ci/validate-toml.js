'use strict';

// Minimaler TOML-Validator fuer die CI/lokale Config-Pruefung.
// Aufruf: node validate-toml.js <module-spec> <file>
//   module-spec: 'toml' oder absoluter Pfad zum toml-Modul.
// Exitcodes: 0 = ok, 2 = Modul fehlt, 3 = Parse-Fehler, 4 = Aufruf-Fehler.

const fs = require('fs');

const spec = process.argv[2];
const file = process.argv[3];

if (!spec || !file) {
  console.error('USAGE: node validate-toml.js <module-spec> <file>');
  process.exit(4);
}

let toml;
try {
  toml = require(spec);
} catch (err) {
  console.error('TOML_MODULE_MISSING: ' + err.message);
  process.exit(2);
}

let text;
try {
  text = fs.readFileSync(file, 'utf8');
} catch (err) {
  console.error('READ_ERROR: ' + err.message);
  process.exit(4);
}

try {
  toml.parse(text);
  process.stdout.write('OK');
} catch (err) {
  console.error('TOML_PARSE_ERROR: ' + err.message);
  process.exit(3);
}
