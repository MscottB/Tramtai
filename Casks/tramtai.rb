# Generado por scripts/paquetes.ps1 al publicar v0.60.0+13112026; no lo edites a mano.
cask "tramtai" do
  version "0.60.0"
  sha256 "b8a45fe2f0e3c1935983117e5838b059eeb13ad3ee3b713ee7d4957483af1404"

  url "https://github.com/MscottB/Tramtai/releases/download/v0.60.0%2B13112026/Tramtai-v0.60.0%2B13112026-macos.dmg"
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
