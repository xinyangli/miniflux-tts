{
  description = "Miniflux TTS integration workspace";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
    in
    {
      devShells = forAllSystems (pkgs:
        let
          go = if pkgs ? go_1_26 then pkgs.go_1_26 else pkgs.go;
        in
        {
          default = pkgs.mkShell {
            packages = [
              go
              pkgs.postgresql
              pkgs.golangci-lint
              pkgs.gopls
              pkgs.jq
              pkgs.curl
              pkgs.netcat
              pkgs.git
            ];
          };
        });

      apps = forAllSystems (pkgs:
        let
          go = if pkgs ? go_1_26 then pkgs.go_1_26 else pkgs.go;
          env = "PATH=${pkgs.lib.makeBinPath [ go pkgs.postgresql pkgs.curl pkgs.jq pkgs.netcat pkgs.git ]}:$PATH";
        in
        {
          tts-test = {
            type = "app";
            program = toString (pkgs.writeShellScript "tts-test" ''
              set -eu
              cd ${self}
              ${env}
              bash ./scripts/tts-test.sh
            '');
          };

          miniflux-test = {
            type = "app";
            program = toString (pkgs.writeShellScript "miniflux-test" ''
              set -eu
              cd ${self}
              ${env}
              bash ./scripts/miniflux-test.sh
            '');
          };

          e2e-test = {
            type = "app";
            program = toString (pkgs.writeShellScript "e2e-test" ''
              set -eu
              cd ${self}
              ${env}
              bash ./scripts/e2e-test.sh
            '');
          };
        });
    };
}
