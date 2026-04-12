{
  description = "GopherTube Nix Flake";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = inputs@{ nixpkgs, flake-parts, flake-utils, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      systems = flake-utils.lib.defaultSystems;

      perSystem = { pkgs, ... }:
        let
          runtimeDeps = with pkgs; [ mpv chafa yt-dlp ];
          gophertubePkg = pkgs.buildGoModule {
            pname = "gophertube";
            version = "2.8.0";
            src = pkgs.lib.cleanSource ./.;
            vendorHash = "sha256-WfVoCxzMk+h4AP1zgTNRXTpj8Ltu71YrsQ7OoU3Y4tg=";

            buildInputs = runtimeDeps;
            nativeBuildInputs = [ pkgs.go pkgs.makeWrapper ] ++ runtimeDeps;
            nativeCheckInputs = [ pkgs.go pkgs.makeWrapper ] ++ runtimeDeps;

            buildPhase = ''
              go build -o gophertube main.go
            '';

            installPhase = ''
              install -Dm755 gophertube $out/bin/gophertube
              wrapProgram $out/bin/gophertube --prefix PATH : ${pkgs.lib.makeBinPath runtimeDeps}
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
