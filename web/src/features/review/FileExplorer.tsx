import { useEffect, useState } from "react";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { hiddenFileNames, isShown } from "@/lib/files";
import { ancestors, buildTree } from "@/lib/tree";
import type { FileEntry, Filter } from "@/lib/types";
import { FileTree } from "./FileTree";

const FILTERS: { value: Filter; label: string }[] = [
  { value: "markdown", label: "Markdown" },
  { value: "all", label: "All files" },
];

interface FileExplorerProps {
  files: FileEntry[];
  filter: Filter;
  selected: string | null;
  onFilterChange: (filter: Filter) => void;
  onSelect: (rel: string) => void;
}

export function FileExplorer({ files, filter, selected, onFilterChange, onSelect }: FileExplorerProps) {
  // Filtering before the tree is built, not after, so a folder holding nothing the filter
  // shows disappears with its contents instead of sitting there empty.
  const tree = buildTree(files.filter((file) => isShown(file, filter)));
  const hidden = hiddenFileNames(files, filter);

  const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set());

  // A deep link arrives with its folders shut. Opening the path to the current file is
  // additive, so it never re-opens a folder the reader has since collapsed.
  useEffect(() => {
    if (!selected) return;
    const path = ancestors(selected);
    if (path.length === 0) return;
    setExpanded((current) => {
      if (path.every((rel) => current.has(rel))) return current;
      return new Set([...current, ...path]);
    });
  }, [selected]);

  function toggle(rel: string) {
    setExpanded((current) => {
      const next = new Set(current);
      if (!next.delete(rel)) next.add(rel);
      return next;
    });
  }

  return (
    <nav
      aria-label="Files"
      className="flex min-h-0 flex-col border-b bg-sidebar text-sidebar-foreground lg:h-full lg:border-r lg:border-b-0"
    >
      <div className="flex items-center gap-2 px-3 py-2">
        <h2 className="text-[0.6875rem] font-medium tracking-widest text-muted-foreground uppercase">Explorer</h2>
        {/* Base UI's Value renders the raw value unless the Root is given the item labels. */}
        <Select items={FILTERS} value={filter} onValueChange={(value) => onFilterChange(value as Filter)}>
          <SelectTrigger
            id="file-filter"
            size="sm"
            aria-label="Which files to show"
            className="ml-auto h-6 border-0 bg-transparent text-xs shadow-none"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {FILTERS.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto pb-2">
        <FileTree
          nodes={tree}
          selected={selected}
          expanded={expanded}
          onSelect={onSelect}
          onToggle={toggle}
        />
      </div>

      {hidden && (
        <p className="border-t px-3 py-2 text-xs text-muted-foreground">
          Hidden, and still handed off: <span className="text-primary">{hidden}</span>
        </p>
      )}
    </nav>
  );
}
