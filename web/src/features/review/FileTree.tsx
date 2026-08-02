import { ChevronDown, ChevronRight, FileText, Folder, FolderOpen } from "lucide-react";
import { type CSSProperties, type KeyboardEvent, type MouseEvent, useEffect, useRef, useState } from "react";
import { cn } from "@/lib/utils";
import { type Row, type TreeNode, flatten } from "@/lib/tree";

interface FileTreeProps {
  nodes: TreeNode[];
  selected: string | null;
  expanded: ReadonlySet<string>;
  onSelect: (rel: string) => void;
  onToggle: (rel: string) => void;
}

const INDENT_REM = 0.75;

function indent(level: number): CSSProperties {
  return { paddingInlineStart: `${(level - 1) * INDENT_REM + 0.375}rem` };
}

// The whole tree is one tab stop (WAI-ARIA's treeview pattern), so Tab reaches the document
// without walking every file first. Inside it the arrow keys move, which is what the pattern
// buys and what an IDE user reaches for anyway.
export function FileTree({ nodes, selected, expanded, onSelect, onToggle }: FileTreeProps) {
  const rows = flatten(nodes, expanded);
  const [focusRel, setFocusRel] = useState<string | null>(null);
  const treeRef = useRef<HTMLUListElement>(null);

  // Filtering or a fresh deep link can retire the focused row; the tab stop then falls back
  // to the open file, and failing that the first row, so the tree is never unreachable.
  const active = rows.find((row) => row.node.rel === focusRel)?.node.rel ?? selected ?? rows[0]?.node.rel ?? null;

  // Only chase focus while the user is already inside the tree — on mount, and on every
  // background refresh of the file list, this must not steal it.
  useEffect(() => {
    const tree = treeRef.current;
    if (!active || !tree?.contains(document.activeElement)) return;
    tree.querySelector<HTMLLIElement>(`[data-rel="${CSS.escape(active)}"]`)?.focus();
  }, [active]);

  function open(node: TreeNode) {
    if (node.kind === "folder") onToggle(node.rel);
    else onSelect(node.rel);
  }

  function handleKeyDown(event: KeyboardEvent<HTMLUListElement>) {
    const index = rows.findIndex((row) => row.node.rel === active);
    const row = rows[index];
    if (!row) return;

    const move = (to: number) => {
      const next = rows[Math.max(0, Math.min(to, rows.length - 1))];
      if (next) setFocusRel(next.node.rel);
    };

    switch (event.key) {
      case "ArrowDown":
        move(index + 1);
        break;
      case "ArrowUp":
        move(index - 1);
        break;
      case "Home":
        move(0);
        break;
      case "End":
        move(rows.length - 1);
        break;
      case "ArrowRight":
        if (row.node.kind !== "folder") return;
        if (expanded.has(row.node.rel)) move(index + 1);
        else onToggle(row.node.rel);
        break;
      case "ArrowLeft":
        if (row.node.kind === "folder" && expanded.has(row.node.rel)) {
          onToggle(row.node.rel);
          break;
        }
        // Otherwise climb: the nearest row above that sits one level shallower is the parent.
        for (let i = index - 1; i >= 0; i--) {
          const candidate = rows[i];
          if (candidate && candidate.level < row.level) {
            setFocusRel(candidate.node.rel);
            break;
          }
        }
        break;
      case "Enter":
      case " ":
        open(row.node);
        break;
      default:
        return;
    }
    event.preventDefault();
  }

  // One delegated handler rather than one per row: a click anywhere in a row, including on
  // a nested group's own rows, resolves to the innermost treeitem it landed in.
  function handleClick(event: MouseEvent<HTMLUListElement>) {
    const rel = (event.target as HTMLElement).closest<HTMLLIElement>("[data-rel]")?.dataset.rel;
    const row = rows.find((candidate) => candidate.node.rel === rel);
    if (!row) return;
    setFocusRel(row.node.rel);
    open(row.node);
  }

  return (
    <ul
      ref={treeRef}
      role="tree"
      aria-label="Files"
      className="flex list-none flex-col p-0"
      onKeyDown={handleKeyDown}
      onClick={handleClick}
    >
      {nodes.map((node) => (
        <TreeItem
          key={node.rel}
          node={node}
          level={1}
          active={active}
          selected={selected}
          expanded={expanded}
        />
      ))}
    </ul>
  );
}

interface TreeItemProps extends Omit<Row, "node"> {
  node: TreeNode;
  active: string | null;
  selected: string | null;
  expanded: ReadonlySet<string>;
}

function TreeItem({ node, level, active, selected, expanded }: TreeItemProps) {
  const isFolder = node.kind === "folder";
  const isOpen = isFolder && expanded.has(node.rel);
  const isSelected = node.rel === selected;

  return (
    <li
      role="treeitem"
      data-rel={node.rel}
      tabIndex={node.rel === active ? 0 : -1}
      aria-level={level}
      aria-expanded={isFolder ? isOpen : undefined}
      aria-selected={isFolder ? undefined : isSelected}
      className="list-none outline-none"
    >
      <div
        style={indent(level)}
        className={cn(
          "flex h-7 cursor-default items-center gap-1.5 pr-2 text-xs select-none",
          "[li:focus-visible>&]:outline-2 [li:focus-visible>&]:-outline-offset-2 [li:focus-visible>&]:outline-sidebar-ring",
          isSelected
            ? "bg-sidebar-primary text-sidebar-primary-foreground"
            : "text-sidebar-foreground hover:bg-sidebar-accent",
        )}
      >
        {isFolder ? (
          <Chevron open={isOpen} />
        ) : (
          <span aria-hidden className="size-3.5 shrink-0" />
        )}
        <Icon node={node} open={isOpen} muted={!isSelected} />
        <span className="truncate">{node.name}</span>
        {node.kind === "file" && node.threads > 0 && (
          <span
            className={cn(
              "ml-auto shrink-0 rounded-full px-1.5 text-[0.6875rem] tabular-nums",
              isSelected ? "bg-sidebar-primary-foreground/20" : "bg-sidebar-accent text-muted-foreground",
            )}
          >
            {node.threads}
          </span>
        )}
      </div>

      {isFolder && isOpen && (
        <ul role="group" className="list-none p-0">
          {node.children.map((child) => (
            <TreeItem
              key={child.rel}
              node={child}
              level={level + 1}
              active={active}
              selected={selected}
              expanded={expanded}
            />
          ))}
        </ul>
      )}
    </li>
  );
}

function Chevron({ open }: { open: boolean }) {
  const Glyph = open ? ChevronDown : ChevronRight;
  return <Glyph aria-hidden className="size-3.5 shrink-0 opacity-70" />;
}

function Icon({ node, open, muted }: { node: TreeNode; open: boolean; muted: boolean }) {
  const Glyph = node.kind === "file" ? FileText : open ? FolderOpen : Folder;
  return <Glyph aria-hidden className={cn("size-3.5 shrink-0", muted && "text-muted-foreground")} />;
}
