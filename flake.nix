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

          gophertubePkg = pkgs.buildGoModule {
            pname = "gophertube";
            version = "2.8.0";
            src = pkgs.lib.cleanSource ./.;
            vendorHash = "";

            buildInputs = with pkgs; [
              mpv
              fzf
              chafa
              yt-dlp
            ];

            nativeBuildInputs = with pkgs; [ go ];
            nativeCheckInputs = with pkgs; [ go ];

            buildPhase = ''
              go build -o gophertube main.go
            '';

            installPhase = ''
              mkdir -p $out/bin
              cp gophertube $out/bin/
              mkdir -p $out/share/man/man1
              cp $src/man/gophertube.1 $out/share/man/man1/
              mkdir -p $out/config
              cp $src/config/gophertube.toml $out/config/gophertube.toml.example
            '';
          };
        in {
          packages.default = gophertubePkg;

          devShells.default = pkgs.mkShell {
            buildInputs = with pkgs; [
              go
              mpv
              fzf
              chafa
              yt-dlp
            ];
          };

          apps.default = flake-utils.lib.mkApp {
            drv = gophertubePkg;
          };
        };
    };
}
