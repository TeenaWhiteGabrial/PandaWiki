import { createRequire } from 'node:module';
import { cp, copyFile, mkdir, readFile } from 'node:fs/promises';
import { dirname, resolve, basename } from 'node:path';
import { fileURLToPath } from 'node:url';

const web = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const app = process.argv[2] || basename(process.cwd());
if (!['admin', 'app'].includes(app)) throw new Error('Expected admin or app');
const require = createRequire(
  resolve(web, 'packages/file-preview/package.json'),
);
const source = dirname(require.resolve('pdfjs-dist/package.json'));
const { version } = JSON.parse(
  await readFile(resolve(source, 'package.json'), 'utf8'),
);
const target = resolve(web, app, 'public/pdfjs');
await mkdir(target, { recursive: true });
// .js gives the module worker the correct MIME type with standard Nginx mime.types.
await copyFile(
  resolve(source, 'build/pdf.worker.min.mjs'),
  resolve(target, 'pdf.worker.min.js'),
);
for (const dir of ['cmaps', 'standard_fonts', 'wasm'])
  await cp(resolve(source, dir), resolve(target, dir), { recursive: true });
await cp(resolve(source, 'LICENSE'), resolve(target, 'LICENSE'));
console.log(`PDF.js ${version}: local assets prepared for ${app}`);
