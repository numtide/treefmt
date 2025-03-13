{
  buildGoModule,
  installShellFiles,
  lib,
  pname,
  src,
  version,
  passthru,
}:
buildGoModule rec {
  inherit pname version src passthru;

  vendorHash = "sha256-A8Gw3kEpw8iYBMw6GSnJdJAaIhZp+zMOoBK4eWwo8lU=";

  subPackages = ["."];

  env.CGO_ENABLED = 0;

  nativeBuildInputs = [
    installShellFiles
  ];

  ldflags = [
    "-s"
    "-w"
    "-X github.com/numtide/treefmt/v2/build.Name=${pname}"
    "-X github.com/numtide/treefmt/v2/build.Version=v${version}"
  ];

  postInstall = ''
    export HOME=$PWD

    installShellCompletion --cmd treefmt \
        --bash <($out/bin/treefmt --completion bash) \
        --fish <($out/bin/treefmt --completion fish) \
        --zsh <($out/bin/treefmt --completion zsh)
  '';

  meta = with lib; {
    description = "treefmt: the formatter multiplexer";
    homepage = "https://github.com/numtide/treefmt";
    license = licenses.mit;
    maintainers = [
      lib.maintainers.brianmcgee
      lib.maintainers.zimbatm
    ];
    mainProgram = "treefmt";
  };
}
