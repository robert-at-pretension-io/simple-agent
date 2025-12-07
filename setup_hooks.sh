#!/bin/sh

HOOK_FILE=".git/hooks/pre-commit"

echo "Installing pre-commit hook to ..."

cat <<HOOK > "$HOOK_FILE"
#!/bin/sh
# Pre-commit hook to verify Go build

echo "🔍 Verifying build before commit..."
if ! go build -o /dev/null .; then
    echo "❌ Build failed! Aborting commit."
    exit 1
fi
echo "✅ Build successful."
HOOK

chmod +x "$HOOK_FILE"
echo "✅ Hook installed."
