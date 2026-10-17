# Generado por scripts/paquetes.ps1 al publicar v0.37.0+16102026; no lo edites a mano.
cask "tramtai" do
  version "0.37.0"
  sha256 "be0a78d3afc8d66f5aaa61e69ee067cb76cde0d9a5965be935203ca32a253ea7"

  url "https://github.com/MscottB/Tramtai/releases/download/v0.37.0%2B16102026/Tramtai-v0.37.0%2B16102026-macos.dmg"
  name "Tramtai"
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"

  depends_on macos: ">= :big_sur"

  app "Tramtai.app"
  binary "#{appdir}/Tramtai.app/Contents/MacOS/tramtai"

  # La app no está firmada por un desarrollador de Apple: sin la marca de cuarentena macOS la abre sin bloquearla.
  postflight do
    system_command "/usr/bin/xattr", args: ["-dr", "com.apple.quarantine", "#{appdir}/Tramtai.app"]
  end

  uninstall launchctl: "com.mscottb.tramtai",
            quit:      "com.mscottb.tramtai"

  zap trash: [
    "~/Library/Application Support/tramtai",
    "~/Library/LaunchAgents/com.mscottb.tramtai.plist",
  ]
end
