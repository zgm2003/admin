set -e
SRC=/d/github-project/infinite-canvas/web
DST=/d/admin/canvas

echo "=== copy components ==="
cp -r "$SRC/src/components/." "$DST/src/components/"

echo "=== copy pages / hooks ==="
cp -r "$SRC/src/pages" "$DST/src/"
cp -r "$SRC/src/hooks" "$DST/src/"

echo "=== copy public assets ==="
cp "$SRC/public/config.js" "$DST/public/"
cp -r "$SRC/public/icons" "$DST/public/"

echo "=== remove replaced scaffold files ==="
rm -f "$DST/src/store/theme.ts" "$DST/src/store/index.ts"
rm -rf "$DST/src/views/static-placeholder"

echo "=== install deps ==="
cd "$DST"
pnpm add @ant-design/pro-components@3.0.0-beta.3 @tanstack/react-query@^5.100.9 axios@^1.16.0 class-variance-authority@^0.7.1 clsx@^2.1.1 copy-to-clipboard@^4.0.2 dayjs@^1.11.20 fflate@^0.8.3 file-saver@^2.0.5 localforage@^1.10.0 motion@^12.38.0 nanoid@^5.1.11 radix-ui@^1.4.3 shadcn@^4.7.0 streamdown@^2.5.0 tailwind-merge@^3.6.0 @uiw/react-codemirror@^4.25.9 @codemirror/lang-json@^6.0.2 @codemirror/lang-javascript@^6.2.5
pnpm add -D @types/file-saver@^2.0.7

echo "=== typecheck ==="
pnpm exec tsc --noEmit 2>&1 | head -100

echo "=== done ==="
