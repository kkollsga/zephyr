// format_fixture.js — deliberately mis-indented to exercise the re-indenter.
function greet(name) {
const parts = {
        first: name,
  greeting: `Hello, ${name}!
this line is inside a template literal { } and must stay put
    keep this weird indent too`,
};

    // A comment with braces { } ( ) [ ] that must not affect nesting.
   if (name) {
return parts.greeting;
        } else {
  return "anonymous";
}
}

const config = {
  retries: 3,
      handlers: [
function () {
        return 1;
},
   () => {
switch (mode) {
case "a":
doThing();
    break;
default:
        doOther();
}
},
],
    regex: /^\{.*\}$/g,
};

greet("world");
