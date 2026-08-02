import type { FileEntry } from "./types";

export type TreeNode =
  | { kind: "file"; name: string; rel: string; threads: number }
  | { kind: "folder"; name: string; rel: string; children: TreeNode[] };

export interface Row {
  node: TreeNode;
  level: number;
}

// A folder's rel is the path prefix it stands for, which makes it unique across the tree and
// usable as both the React key and the expansion-set key.
function folderAt(children: TreeNode[], rel: string): Extract<TreeNode, { kind: "folder" }> {
  const name = rel.slice(rel.lastIndexOf("/") + 1);
  const existing = children.find((child) => child.kind === "folder" && child.rel === rel);
  if (existing?.kind === "folder") return existing;

  const folder: Extract<TreeNode, { kind: "folder" }> = { kind: "folder", name, rel, children: [] };
  children.push(folder);
  return folder;
}

// Folders before files, each group alphabetical — the explorer convention, and not what the
// server sends: its flat sort puts "a.md" before "a/b.md" because "." sorts below "/".
function sortNodes(nodes: TreeNode[]) {
  nodes.sort((a, b) => {
    if (a.kind !== b.kind) return a.kind === "folder" ? -1 : 1;
    return a.name.localeCompare(b.name);
  });
  for (const node of nodes) {
    if (node.kind === "folder") sortNodes(node.children);
  }
}

export function buildTree(files: FileEntry[]): TreeNode[] {
  const roots: TreeNode[] = [];

  for (const file of files) {
    const segments = file.rel.split("/");
    const name = segments.pop();
    if (!name) continue;

    let children = roots;
    let prefix = "";
    for (const segment of segments) {
      prefix = prefix ? `${prefix}/${segment}` : segment;
      children = folderAt(children, prefix).children;
    }
    children.push({ kind: "file", name, rel: file.rel, threads: file.threads });
  }

  sortNodes(roots);
  return roots;
}

// The rows a keyboard user can actually move between: depth-first, skipping the subtrees of
// collapsed folders. The tree's arrow keys walk this list rather than the DOM.
export function flatten(nodes: TreeNode[], expanded: ReadonlySet<string>, level = 1): Row[] {
  const rows: Row[] = [];
  for (const node of nodes) {
    rows.push({ node, level });
    if (node.kind === "folder" && expanded.has(node.rel)) {
      rows.push(...flatten(node.children, expanded, level + 1));
    }
  }
  return rows;
}

// Every folder rel above a file, so a deep link can open with its path already expanded.
export function ancestors(rel: string): string[] {
  const segments = rel.split("/");
  segments.pop();
  return segments.map((_, i) => segments.slice(0, i + 1).join("/"));
}
