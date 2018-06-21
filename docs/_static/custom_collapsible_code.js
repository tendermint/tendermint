let makeCodeBlocksCollapsible = function() {
    $(".toggle > *").hide();
    $(".toggle .header").show();
    $(".toggle .header").click(function() {
        $(this).parent().children().not(".header").toggle({"duration": 400});
        $(this).parent().children(".header").toggleClass("open");
    });
};
// we could use the }(); way if we would have access to jQuery in HEAD, i.e. we would need to force the theme
// to load jQuery before our custom scripts
