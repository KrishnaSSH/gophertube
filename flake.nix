{
  description = "Description for the project";

  inputs = {
    flake-parts.url = "github:hercules-ci/flake-parts";
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = inputs@{ flake-parts, ... }:
    flake-parts.lib.mkFlake { inherit inputs; } {
      imports = [

      ];
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin" ];
      perSystem = { pkgs, ... }: {
        packages.default = pkgs.buildGoModule {
          pname = "gophertube";
          version = "1.2.1";
          src = pkgs.lib.cleanSource ./.;
          vendorHash = "sha256-WfVoCxzMk+h4AP1zgTNRXTpj8Ltu71YrsQ7OoU3Y4tg=";

          buildInputs = with pkgs; [
            mpv
            fzf
            chafa
            yt-dlp
          ];

          nativeBuildInputs = with pkgs; [
            go
          ];
          nativeCheckInputs = with pkgs; [
            go
          ];

          buildPhase = ''
            go build -o gophertube main.go
          '';

          installPhase = ''
            mkdir -p $out/bin
            cp gophertube $out/bin
            mkdir -p $out/share/man/man1
            cp $src/man/gophertube.1 $out/share/man/man1
            mkdir -p $out/config
            cp $src/config/gophertube.toml $out/config/gophertube.toml.example
          '';
        };
        devShells.default = pkgs.mkShell {
          nativeBuildInputs = with pkgs; [
            go
          ];
          buildInputs = with pkgs; [
            mpv
            fzf
            chafa
          ];
        };
      };
      flake = {

      };
    };
}
