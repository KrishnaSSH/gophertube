{
  description = "GopherTube Nix Flake";
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs = inputs@{ nixpkgs, flake-parts, flake-utils, self, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = flake-utils.lib.defaultSystems;
      perSystem = { pkgs, ... }:
        let
          version = if self ? rev
            then builtins.substring 0 8 self.rev
            else self.lastModifiedDate or "unknown";
          tag = if self ? tags
            then self.tags
            else "dev";
          runtimeDeps = with pkgs; [ mpv yt-dlp ];
          gophertubePkg = pkgs.buildGoModule {
            pname = "gophertube";
            version = tag;
            src = pkgs.lib.cleanSource ./.;
            vendorHash = "sha256-WfVoCxzMk+h4AP1zgTNRXTpj8Ltu71YrsQ7OoU3Y4tg=";
            CGO_ENABLED = 0;
            ldflags = [
              "-s" "-w"
              "-X gophertube/internal/app.version=${tag}"
            ];
            nativeBuildInputs = [ pkgs.makeWrapper ];
            postInstall = ''
              wrapProgram $out/bin/gophertube \
                --prefix PATH : ${pkgs.lib.makeBinPath runtimeDeps}
              mkdir -p $out/share/man/man1
              cp $src/man/gophertube.1 $out/share/man/man1/
              mkdir -p $out/config
              cp $src/config/gophertube.toml $out/config/gophertube.toml.example
            '';
          };
        in {
          packages.default = gophertubePkg;
          devShells.default = pkgs.mkShell {
            buildInputs = [ pkgs.go ] ++ runtimeDeps;
          };
          apps.default = flake-utils.lib.mkApp {
            drv = gophertubePkg;
          };
        };
    };
}
