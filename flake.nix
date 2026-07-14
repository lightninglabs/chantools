{
  description = "Lightning Labs chantools";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs =
    {
      self,
      nixpkgs,
    }:
    let
      systems = [
        "aarch64-darwin"
        "aarch64-linux"
        "x86_64-darwin"
        "x86_64-linux"
      ];

      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forAllSystems (
        system:
        let
          pkgs = import nixpkgs { inherit system; };
        in
        {
          default = pkgs.buildGo125Module {
            pname = "chantools";
            version = "0.14.2";

            src = ./.;
            vendorHash = "sha256-vrmGXMWGj+uy5913xiNiDUaY8plLoycwDLln0RsKv18=";

            subPackages = [ "cmd/chantools" ];
            env.CGO_ENABLED = "0";

            meta = {
              description = "Tools for recovering funds from Lightning channels";
              homepage = "https://github.com/lightninglabs/chantools";
              license = pkgs.lib.licenses.mit;
              mainProgram = "chantools";
            };
          };

          chantools = self.packages.${system}.default;
        }
      );
    };
}
