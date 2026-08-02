import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { hiddenFileNames, isShown } from "@/lib/files";
import type { FileEntry, Filter } from "@/lib/types";

interface FileExplorerProps {
  files: FileEntry[];
  filter: Filter;
  onFilterChange: (filter: Filter) => void;
}

export function FileExplorer({ files, filter, onFilterChange }: FileExplorerProps) {
  const shown = files.filter((file) => isShown(file, filter));
  const hidden = hiddenFileNames(files, filter);

  return (
    <nav aria-label="Files" className="rounded-xl border bg-card p-4 text-sm shadow-xs">
      <div className="flex items-center gap-2">
        <Label htmlFor="file-filter">Show</Label>
        <Select value={filter} onValueChange={(value) => onFilterChange(value as Filter)}>
          <SelectTrigger id="file-filter" size="sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="markdown">Markdown</SelectItem>
            <SelectItem value="all">All files</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <ul className="mt-2 flex list-none flex-col gap-0.5 p-0">
        {shown.map((file) => (
          <li key={file.rel} className="flex items-baseline gap-2">
            <span className="truncate">{file.rel}</span>
            {file.threads > 0 && (
              <span className="ml-auto shrink-0 rounded-full border bg-background px-1.5 text-xs text-muted-foreground">
                {file.threads}
              </span>
            )}
          </li>
        ))}
      </ul>

      {hidden && <p className="mt-2 text-primary">Hidden, and still handed off: {hidden}</p>}
    </nav>
  );
}
