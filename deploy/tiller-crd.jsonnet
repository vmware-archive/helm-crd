// CRD and tiller, with controller running as a sidecar

local utils = import "utils.libsonnet";
local tiller = import "tiller.jsonnet";

// Run CRD controller as a sidecar, and restrict tiller port to pod-only
local controller_overlay = {
  spec+: {
    template+: {
      spec+: {
        volumes+: [
          // Used as temporary space while downloading charts, etc.
          {name: "home", emptyDir: {}},
        ],
        containers: [
          super.containers[0] {
            assert self.name == "tiller",
            // Nuke exposed tiller port
            ports: [], // Informational only
            args+: ["--listen=localhost:44134"],  // Restrict to pod only
          },
          {
            name: "controller",
            image: "bitnami/helm-crd-controller:latest",
            securityContext: {
              readOnlyRootFilesystem: true,
            },
            command: ["/controller"],
            args: [
              "--home=/helm",
              "--host=localhost:44134",
            ],
            env: [
              {name: "TMPDIR", value: "/helm"},
            ],
            volumeMounts: [
              {name: "home", mountPath: "/helm"},
            ],
          },
        ],
      },
    },
  },
};

{
  crd: utils.CustomResourceDefinition("helm.bitnami.com", "v1", "HelmRelease"),

  tiller: tiller + controller_overlay,
}
