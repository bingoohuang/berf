function renderView(dict) {
    for (let key in dict.values) {
        let view = views[key];
        if (!view) {
            continue;
        }

        let arr = dict.values[key];
        if (arr == null) {
            continue
        }

        let opt = view.getOption();
        let x = opt.xAxis[0].data;
        x.push(dict.time);
        opt.xAxis[0].data = x;

        for (let i = 0; i < arr.length; i++) {
            let y = opt.series[i].data;
            y.push({value: arr[i]});
            opt.series[i].data = y;
        }
        view.setOption(opt);
    }
}

function renderViewPoints(arr, from) {
    let to = from + 1;
    if (to > arr.length) {
        to = arr.length;
    }

    for (let i = from; i < to; i++) {
        renderView(arr[i])
    }

    if (to < arr.length) {
        setTimeout(function () {
            renderViewPoints(arr, to)
        }, 10)
    }
}

function views_sync() {
    $.ajax({
        type: "GET",
        url: "/data/",
        dataType: "json",
        success: function (arr) {
            renderViewPoints(arr, 0);
        }
    });
}
