$(document).ready(function () {
    $(".myForm").each(function () {
        const $form = $(this)
        const $button = $(this).find(".myFormSubmit")
        const $result = $(this).find(".myFormResult")
        $button.click(function () {
            const action = $form.attr("data-action")
            const values = {}
            $form.find(".myFormInput").each(function () {
                const $input = $(this)
                const key = $input.attr("name")
                let value = $input.val()
                if ($input.attr("type") === "number") {
                    value = parseInt(value)
                }
                values[key] = value
            })
            $button.prop('disabled', true);
            $.ajax({
                type: "POST",
                url: `/${action}`,
                data: JSON.stringify(values),
                dataType: "json",
                contentType: 'application/json',
                success(data) {
                    $result.text(JSON.stringify(data, null, 2)).css("color", "black").show()
                },
                error(xhr, status, error) {
                    $result.text(xhr.responseText).css("color", "red").show()
                },
                complete() {
                    $button.prop('disabled', false);
                },
            });
        })
    });
})