const fs = require('fs');
const path = require('path');
const { execSync } = require('child_process');
function sha() {
  try {
    return execSync('git rev-parse --short HEAD', { cwd: path.join(__dirname, '..') }).toString().trim();
  } catch {
    return 'unknown';
  }
}
const s = sha();
const root = path.join(__dirname, '..'); // web/ (scripts -> parent)
fs.mkdirSync(path.join(root, 'src/lib'), { recursive: true });
fs.mkdirSync(path.join(root, 'public'), { recursive: true });
fs.writeFileSync(path.join(root, 'src/lib/version.generated.ts'), `export const BUILD_VERSION = "${s}";\n`);
fs.writeFileSync(path.join(root, 'public/version.json'), JSON.stringify({ version: s }) + '\n');
console.log('gen-version:', s);
