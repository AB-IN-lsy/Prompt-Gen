# TODO 列表

1. 现在有bug，注册的时候上传头像失败，显示 missing authorization header。但是我注册完成之后，在用户界面上传头像，是成功的
2. 然后就是验证邮件的问题，邮件不太对
   1. 比如 下面是邮件正文，但是邮箱验证按钮无法点击，然后这个 link ：<https://prompt.ab-in.cn/email/verified?token=ea4c72f7-ec97-47d1-ab6f-eed8ac1b34f7> 是空白页面，没有验证的东西

    ```text
    Hello sliu424,

    Welcome to PromptGen! Please verify your email by clicking the button below:

    Verify Email / 邮箱验证

    Or copy the link:
    =https://prompt.ab-in.cn/email/verified?token=ea4c72f7-ec97-47d1-ab6f-eed8ac1b34f7
    ```

3. 刚刚的问题都是上线到服务器产生的问题
