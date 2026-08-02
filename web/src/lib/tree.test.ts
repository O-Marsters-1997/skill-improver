import { describe, expect, test } from "bun:test";
import { ancestors, buildTree, flatten } from "./tree";
import type { FileEntry } from "./types";

function entries(...rels: string[]): FileEntry[] {
  return rels.map((rel) => ({ rel, ext: ".md", threads: 0 }));
}

function names(rows: ReturnType<typeof flatten>) {
  return rows.map((row) => `${"  ".repeat(row.level - 1)}${row.node.name}`);
}

const all = (rels: string[]) => new Set(rels);

describe("buildTree", () => {
  test("nests on path segments and reuses a folder across siblings", () => {
    const tree = buildTree(entries("references/cli.md", "references/theming.md", "SKILL.md"));

    expect(names(flatten(tree, all(["references"])))).toEqual([
      "references",
      "  cli.md",
      "  theming.md",
      "SKILL.md",
    ]);
  });

  test("folders sort before files, which the server's flat sort does not do", () => {
    // "a.md" < "a/b.md" as plain strings ("." is 0x2e, "/" is 0x2f), so a tree built
    // straight off the server order would interleave the folder with its neighbours.
    const flat = entries("a.md", "a/b.md", "b.md");
    expect([...flat].map((f) => f.rel).sort()).toEqual(["a.md", "a/b.md", "b.md"]);

    expect(names(flatten(buildTree(flat), all(["a"])))).toEqual(["a", "  b.md", "a.md", "b.md"]);
  });

  test("nests arbitrarily deep", () => {
    const tree = buildTree(entries("a/b/c/d.md"));
    expect(names(flatten(tree, all(["a", "a/b", "a/b/c"])))).toEqual(["a", "  b", "    c", "      d.md"]);
  });
});

describe("flatten", () => {
  const tree = buildTree(entries("references/cli.md", "references/nested/deep.md", "SKILL.md"));

  test("hides the subtree of a collapsed folder", () => {
    expect(names(flatten(tree, new Set()))).toEqual(["references", "SKILL.md"]);
  });

  test("stops at the deepest expanded folder", () => {
    expect(names(flatten(tree, all(["references"])))).toEqual([
      "references",
      "  nested",
      "  cli.md",
      "SKILL.md",
    ]);
  });

  test("levels are 1-based, matching aria-level", () => {
    expect(flatten(tree, all(["references"])).map((row) => row.level)).toEqual([1, 2, 2, 1]);
  });
});

describe("ancestors", () => {
  test("names every folder above a file, outermost first", () => {
    expect(ancestors("a/b/c.md")).toEqual(["a", "a/b"]);
  });

  test("a root file has none", () => {
    expect(ancestors("SKILL.md")).toEqual([]);
  });
});
