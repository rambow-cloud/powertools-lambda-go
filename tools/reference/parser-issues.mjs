// Preserve portable diagnostics recursively; constraint-specific metadata is a separate gate.
function project(issue) {
 return {code:issue.code,message:issue.message,
  ...(issue.path?.length?{path:issue.path}:{}),
  ...(typeof issue.expected==='string'?{expected:issue.expected}:{}),
  ...(Array.isArray(issue.errors)?{errors:issue.errors.map(branch=>branch.map(project))}:{})};
}
export function issues(error) {
 return (Array.isArray(error?.cause)?error.cause:error?.cause?.issues??error?.issues??[]).map(project);
}
