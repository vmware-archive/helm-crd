local remap(v, start, end, newstart) =
  if v >= start && v <= end then v - start + newstart else v;

local remapChar(c, start, end, newstart) =
  std.char(remap(
    std.codepoint(c), std.codepoint(start), std.codepoint(end), std.codepoint(newstart)));

{
  toLower(s):: (
    std.join("", [remapChar(c, "A", "Z", "a") for c in std.stringChars(s)])
  ),

  CustomResourceDefinition(group, version, kind):: {
    local this = self,
    apiVersion: "apiextensions.k8s.io/v1beta1",
    kind: "CustomResourceDefinition",
    metadata+: {
      name: this.spec.names.plural + "." + this.spec.group,
    },
    spec: {
      scope: "Namespaced",
      group: group,
      version: version,
      names: {
        kind: kind,
        singular: $.toLower(self.kind),
        plural: self.singular + "s",
        listKind: self.kind + "List",
      },
    },
  },
}
