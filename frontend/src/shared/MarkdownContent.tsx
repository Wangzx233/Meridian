import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

export function MarkdownContent(props: { children: string; compact?: boolean }) {
  return (
    <div className={`markdownContent ${props.compact ? "isCompact" : ""}`}>
      <ReactMarkdown
        skipHtml
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ node: _node, ...linkProps }) => <a {...linkProps} target="_blank" rel="noreferrer" />,
          input: ({ node: _node, ...inputProps }) => <input {...inputProps} disabled />,
        }}
      >
        {props.children}
      </ReactMarkdown>
    </div>
  );
}
