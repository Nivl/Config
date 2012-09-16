import sublime, sublime_plugin


class ReplaceTabCommand(sublime_plugin.TextCommand):
    def run(self, content):
        tab_size = self.view.settings().get("tab_size")
        self.view.settings().set("tab_size", 8);
        self.view.run_command("expand_tabs")
        self.view.settings().set("tab_size", tab_size);