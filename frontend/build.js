// 简单的前端构建脚本：将静态文件复制到 dist 目录
// TickToken 前端是纯 HTML/CSS/JS，无需打包工具
const fs = require('fs');
const path = require('path');

const srcDir = __dirname;
const distDir = path.join(srcDir, 'dist');

// 清理并重建 dist 目录
if (fs.existsSync(distDir)) {
  fs.rmSync(distDir, { recursive: true, force: true });
}
fs.mkdirSync(distDir, { recursive: true });

// 复制 index.html
fs.copyFileSync(path.join(srcDir, 'index.html'), path.join(distDir, 'index.html'));
console.log('Copied index.html');

// 复制 src 目录
const srcSubDir = path.join(srcDir, 'src');
if (fs.existsSync(srcSubDir)) {
  copyDir(srcSubDir, path.join(distDir, 'src'));
  console.log('Copied src/');
}

// 复制 wailsjs 目录（运行时绑定）
const wailsjsDir = path.join(srcDir, 'wailsjs');
if (fs.existsSync(wailsjsDir)) {
  copyDir(wailsjsDir, path.join(distDir, 'wailsjs'));
  console.log('Copied wailsjs/');
}

// 保留 .gitkeep 让 dist 目录可被 git 跟踪（embed 需要）
fs.writeFileSync(path.join(distDir, '.gitkeep'), '');
console.log('Build complete: dist/ created');

// 递归复制目录
function copyDir(src, dest) {
  if (!fs.existsSync(dest)) {
    fs.mkdirSync(dest, { recursive: true });
  }
  const entries = fs.readdirSync(src, { withFileTypes: true });
  for (const entry of entries) {
    const srcPath = path.join(src, entry.name);
    const destPath = path.join(dest, entry.name);
    if (entry.isDirectory()) {
      copyDir(srcPath, destPath);
    } else {
      fs.copyFileSync(srcPath, destPath);
    }
  }
}
