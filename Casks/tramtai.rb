# Generado por scripts/paquetes.ps1 al publicar v0.58.0+31102026; no lo edites a mano.
cask "tramtai" do
  version "0.58.0"
  sha256 "0fd2d757b18403a6b23fd6034ee0855cefeda8c65c81905db9b007d6d7c83202"

  url "https://github.com/MscottB/Tramtai/releases/download/v0.58.0%2B31102026/Tramtai-v0.58.0%2B31102026-macos.dmg"
  name "Tramtai"
  desc "Tus agentes de IA (Claude Code, Codex, la API de Gemini) como una API local compatible con OpenAI y Anthropic"
  homepage "https://mscottb.github.io/Tramtai/"

  depends_on macos: :big_sur

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
